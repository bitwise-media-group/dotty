// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// Tree maps one repository directory onto the live directory its entries are
// linked into, e.g. <repo>/home/.config onto ~/.config. TargetPerm is applied
// to Target when Apply has to create it (0o755 when zero) — ~/.ssh wants
// 0o700. Copy lists Target-relative directories deployed as real directories
// of file copies instead of symlinks, for tools that refuse symlinked
// configuration: grok's sandbox write-denies its hooks directory and will not
// start when the directory or any hook path component is a symlink, since a
// symlink would let sandboxed code retarget the hooks past the deny.
type Tree struct {
	Source     string
	Target     string
	TargetPerm os.FileMode
	Copy       []string
}

// State classifies what linking an entry requires.
type State int

const (
	// StateLink means nothing occupies the site; a symlink will be created.
	StateLink State = iota
	// StateOK means a symlink already points at the wanted destination.
	StateOK
	// StateRelink means a symlink points elsewhere or dangles; it is replaced
	// without backup — removing a symlink never destroys the file it names.
	StateRelink
	// StateConflict means a real file (or a directory where the source is a
	// file) occupies the site; a Resolver decides its fate.
	StateConflict
)

// Action is one entry's planned link: the site under Target and the
// destination under Source it should point at.
type Action struct {
	Site  string
	Dest  string
	State State
	// Generated marks a destination dotty renders from the profile's answers
	// (per-profile files, the profile directory itself). Adopting over one
	// would silently revert a fresh render, so resolvers refuse it.
	Generated bool
	// Copy marks a site from Tree.Copy: it is materialized as a real
	// directory mirroring Dest's files, never a symlink. Its states read as
	// missing (StateLink), out of sync or still a symlink (StateRelink), in
	// sync (StateOK), and a regular file in the way (StateConflict).
	Copy bool
}

// Conflict is a real filesystem entry occupying a link site.
type Conflict struct {
	Site      string // the existing file or directory in the way
	Dest      string // what the symlink would point at
	IsDir     bool
	Generated bool // the destination is machine-generated; adoption is refused
}

// Resolution is a Resolver's decision for one Conflict.
type Resolution int

const (
	// ResBackup moves the entry under the backup root, then links.
	ResBackup Resolution = iota
	// ResAdopt moves the entry into the repository over the source copy, then
	// links — the user's existing file wins and lands in git for review.
	ResAdopt
	// ResSkip leaves the entry untouched and records it in the Report.
	ResSkip
	// ResFail aborts Apply with an error naming the site.
	ResFail
)

// Resolver decides what Apply does with a conflicting real file.
type Resolver func(Conflict) (Resolution, error)

// Report summarizes one Apply: sites linked fresh, stale symlinks replaced,
// copy-deployed directories created or resynced, conflicts backed up or
// adopted before linking, conflicts skipped, how many links were already
// correct, and legacy shadows retired into the backup set.
type Report struct {
	Linked   []string
	Replaced []string
	Copied   []string
	Backed   []string
	Adopted  []string
	Skipped  []string
	Retired  []string
	OK       int
}

// Plan walks tree and reports what Apply would do, without touching anything.
// Conflicts surface as StateConflict actions.
func Plan(tree Tree) ([]Action, error) {
	var actions []Action
	err := walk(tree.Source, tree.Target, copySites(tree), func(a Action) error {
		actions = append(actions, a)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return actions, nil
}

// copySites resolves tree.Copy to the absolute sites walk matches against.
func copySites(tree Tree) map[string]bool {
	sites := make(map[string]bool, len(tree.Copy))
	for _, rel := range tree.Copy {
		sites[filepath.Join(tree.Target, rel)] = true
	}
	return sites
}

// Status is Plan under a name that reads as the query it is.
func Status(tree Tree) ([]Action, error) { return Plan(tree) }

// Apply links tree into its target, resolving each conflicting real file
// through resolve. Backed-up entries land under backupRoot at a mirror of
// their absolute path (backupRoot/Users/x/.config/foo), so a backup set is
// restorable without a manifest.
func Apply(tree Tree, resolve Resolver, backupRoot string) (Report, error) {
	var rep Report
	perm := tree.TargetPerm
	if perm == 0 {
		perm = 0o755
	}
	if err := cli.EnsureDir(tree.Target, perm); err != nil {
		return rep, err
	}

	err := walk(tree.Source, tree.Target, copySites(tree), func(a Action) error {
		return apply(a, resolve, backupRoot, &rep)
	})
	return rep, err
}

// ApplyFile links one site at dest with the same semantics Apply gives tree
// entries — for links whose destination lives outside the tree, like the
// machine-rendered files under the active profile. Those destinations are
// generated, so a conflicting real file is never adopted over them. The
// site's parent must exist.
func ApplyFile(site, dest string, resolve Resolver, backupRoot string, rep *Report) error {
	a := Action{Site: site, Dest: dest, State: StateConflict, Generated: true}
	info, err := os.Lstat(site)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		a.State = StateLink
	case err != nil:
		return fmt.Errorf("inspect %s: %w", site, err)
	case info.Mode()&os.ModeSymlink != 0:
		a.State = StateRelink
		if existing, rerr := os.Readlink(site); rerr == nil && existing == dest {
			a.State = StateOK
		}
	}
	return apply(a, resolve, backupRoot, rep)
}

// apply executes one classified Action, recording it in rep.
func apply(a Action, resolve Resolver, backupRoot string, rep *Report) error {
	if a.Copy {
		return applyCopy(a, resolve, backupRoot, rep)
	}
	switch a.State {
	case StateOK:
		rep.OK++
		return nil
	case StateLink:
		if err := os.Symlink(a.Dest, a.Site); err != nil {
			return fmt.Errorf("link %s: %w", a.Site, err)
		}
		rep.Linked = append(rep.Linked, a.Site)
		return nil
	case StateRelink:
		if err := os.Remove(a.Site); err != nil {
			return fmt.Errorf("remove stale symlink %s: %w", a.Site, err)
		}
		if err := os.Symlink(a.Dest, a.Site); err != nil {
			return fmt.Errorf("link %s: %w", a.Site, err)
		}
		rep.Replaced = append(rep.Replaced, a.Site)
		return nil
	}

	info, err := os.Stat(a.Site)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", a.Site, err)
	}
	res, err := resolve(Conflict{Site: a.Site, Dest: a.Dest, IsDir: info.IsDir(), Generated: a.Generated})
	if err != nil {
		return err
	}
	// A generated destination was just rendered from the profile's answers;
	// moving the stale site over it would silently revert that render.
	if res == ResAdopt && a.Generated {
		res = ResBackup
	}
	switch res {
	case ResBackup:
		dst := filepath.Join(backupRoot, strings.TrimPrefix(a.Site, string(filepath.Separator)))
		if err := move(a.Site, dst); err != nil {
			return fmt.Errorf("back up %s: %w", a.Site, err)
		}
		rep.Backed = append(rep.Backed, a.Site)
	case ResAdopt:
		if err := os.RemoveAll(a.Dest); err != nil {
			return fmt.Errorf("adopt %s: drop source copy: %w", a.Site, err)
		}
		if err := move(a.Site, a.Dest); err != nil {
			return fmt.Errorf("adopt %s: %w", a.Site, err)
		}
		rep.Adopted = append(rep.Adopted, a.Site)
	case ResSkip:
		rep.Skipped = append(rep.Skipped, a.Site)
		return nil
	default:
		return fmt.Errorf("%s: existing file conflicts with %s", a.Site, a.Dest)
	}
	if err := os.Symlink(a.Dest, a.Site); err != nil {
		return fmt.Errorf("link %s: %w", a.Site, err)
	}
	return nil
}

// walk pairs each source entry with its link site, descending only where the
// site is already a real directory (so whole directories link folded until
// something unfolds them) and emitting one Action everywhere else. Sites in
// copies are never descended into or linked; they emit one copy Action for
// the whole directory.
func walk(source, target string, copies map[string]bool, visit func(Action) error) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	for _, e := range entries {
		dest := filepath.Join(source, e.Name())
		site := filepath.Join(target, e.Name())

		if copies[site] && e.IsDir() {
			a, err := classifyCopy(site, dest)
			if err == nil {
				err = visit(a)
			}
			if err != nil {
				return err
			}
			continue
		}

		info, err := os.Lstat(site)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			err = visit(Action{Site: site, Dest: dest, State: StateLink})
		case err != nil:
			err = fmt.Errorf("inspect %s: %w", site, err)
		case info.Mode()&os.ModeSymlink != 0:
			state := StateRelink
			if existing, rerr := os.Readlink(site); rerr == nil && existing == dest {
				state = StateOK
			}
			err = visit(Action{Site: site, Dest: dest, State: state})
		case info.IsDir() && e.IsDir():
			err = walk(dest, site, copies, visit)
		default:
			err = visit(Action{Site: site, Dest: dest, State: StateConflict})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// move renames path to dst, creating dst's parents. Backup and adoption stay
// on the same volume in practice, so a plain rename is enough; a cross-device
// move surfaces as the wrapped EXDEV rather than a silent partial copy.
func move(path, dst string) error {
	if err := cli.EnsureDir(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(path, dst); err != nil {
		return fmt.Errorf("move %s to %s: %w", path, dst, err)
	}
	return nil
}
