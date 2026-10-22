// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bitwise-media-group/dotty/internal/mise"
)

type miseDeployment struct {
	client *mise.Client
	home   string
}

func (d *miseDeployment) tree(ctx context.Context, tree Tree, resolve Resolver, backup string, rep *Report) error {
	actions, err := Plan(tree)
	if err != nil {
		return err
	}
	return d.apply(ctx, actions, resolve, backup, rep)
}

func (d *miseDeployment) file(
	ctx context.Context, site, source string, resolve Resolver, backup string, rep *Report,
) error {
	a := Action{Site: site, Dest: source, State: StateLink, Generated: true}
	info, err := os.Lstat(site)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", site, err)
	}
	if err == nil {
		a.State = StateConflict
		if info.Mode()&os.ModeSymlink != 0 {
			a.State = StateRelink
			if link, err := os.Readlink(site); err == nil && link == source {
				a.State = StateOK
			}
		}
	}
	return d.apply(ctx, []Action{a}, resolve, backup, rep)
}

// apply leaves conflict choices and the existing backup format with dotty,
// then hands the prepared targets to mise. Declarations live in the user
// config so mise add/untrack can edit them rather than a generated fragment
// masking the user's changes. Registering does not capture live files over
// the repository's sources.
func (d *miseDeployment) apply(
	ctx context.Context, actions []Action, resolve Resolver, backup string, rep *Report,
) error {
	managed, err := d.client.DotfileTargets()
	if err != nil {
		return err
	}
	var lines, targets []string
	for _, a := range actions {
		mode := "symlink"
		if a.Copy || d.isGrok(a.Site) {
			mode = "copy"
			a.Copy = true
			// A managed copy can be updated without treating every re-link
			// as a conflict. A symlink from the old layout must be removed
			// before copying so the source is never written through it.
			if info, err := os.Lstat(a.Site); err == nil && info.Mode()&os.ModeSymlink != 0 {
				a.State = StateRelink
			} else if err == nil && info.Mode().IsRegular() {
				source, err := os.Stat(a.Dest)
				if err != nil {
					return fmt.Errorf("inspect %s: %w", a.Dest, err)
				}
				if !source.Mode().IsRegular() {
					a.State = StateConflict
				} else {
					a.State = StateConflict
					if managed[d.portable(a.Site)] || managed[a.Site] {
						a.State = StateRelink
					}
					if same, err := fileInSync(a.Dest, a.Site); err != nil {
						return err
					} else if same {
						a.State = StateOK
					}
				}
			}
		}
		lines = append(lines, fmt.Sprintf("%q = { source = %q, mode = %q }", d.portable(a.Site), d.portable(a.Dest), mode))
		if err := prepareMise(a, resolve, backup, rep); err != nil {
			return err
		}
		if !slices.Contains(rep.Skipped, a.Site) {
			targets = append(targets, a.Site)
		}
	}
	if err := d.client.RegisterDotfiles(lines); err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil // no targets means apply everything to mise
	}
	return d.client.Dotfiles(ctx, append([]string{"apply", "--yes"}, targets...)...)
}

func (d *miseDeployment) portable(path string) string {
	if rel, ok := strings.CutPrefix(path, d.home+string(filepath.Separator)); ok {
		return "~/" + filepath.ToSlash(rel)
	}
	return path
}

func (d *miseDeployment) isGrok(path string) bool {
	root := filepath.Join(d.home, ".config", "grok")
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func prepareMise(a Action, resolve Resolver, backup string, rep *Report) error {
	switch a.State {
	case StateOK:
		rep.OK++
		return nil
	case StateRelink:
		if info, err := os.Lstat(a.Site); err == nil && info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(a.Site); err != nil {
				return fmt.Errorf("remove stale symlink %s: %w", a.Site, err)
			}
		}
		rep.Replaced = append(rep.Replaced, a.Site)
	case StateLink:
		rep.Linked = append(rep.Linked, a.Site)
	case StateConflict:
		info, err := os.Stat(a.Site)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", a.Site, err)
		}
		res, err := resolve(Conflict{Site: a.Site, Dest: a.Dest, IsDir: info.IsDir(), Generated: a.Generated})
		if err != nil {
			return err
		}
		if res == ResAdopt && a.Generated {
			res = ResBackup
		}
		switch res {
		case ResSkip:
			rep.Skipped = append(rep.Skipped, a.Site)
			return nil
		case ResBackup:
			dst := filepath.Join(backup, strings.TrimPrefix(a.Site, string(filepath.Separator)))
			if err := move(a.Site, dst); err != nil {
				return err
			}
			rep.Backed = append(rep.Backed, a.Site)
		case ResAdopt:
			if err := os.RemoveAll(a.Dest); err != nil {
				return fmt.Errorf("adopt %s: drop source copy: %w", a.Site, err)
			}
			if err := move(a.Site, a.Dest); err != nil {
				return err
			}
			rep.Adopted = append(rep.Adopted, a.Site)
		default:
			return fmt.Errorf("%s: existing file conflicts with %s", a.Site, a.Dest)
		}
	}
	if a.Copy {
		rep.Copied = append(rep.Copied, a.Site)
	}
	return nil
}
