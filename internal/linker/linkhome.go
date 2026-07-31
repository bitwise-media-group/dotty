// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/scaffold"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// LinkHome pre-creates the unfold directories and links the repository's
// home tree over $HOME, the repository's profiles under the dotty config
// directory, and the per-profile files through the active-profile symlink.
// It returns the report and the backup directory conflicts were mirrored
// into.
func LinkHome(ios cli.IOStreams, a scaffold.Answers, repo, home, onConflict string) (Report, string, error) {
	unfold, err := scaffold.Unfold(a)
	if err != nil {
		return Report{}, "", err
	}
	for _, rel := range unfold {
		perm := os.FileMode(0o755)
		if rel == ".ssh" {
			perm = 0o700
		}
		if err := cli.EnsureDir(filepath.Join(home, rel), perm); err != nil {
			return Report{}, "", err
		}
	}

	dataDir, err := cli.DataDir()
	if err != nil {
		return Report{}, "", err
	}
	backupDir := filepath.Join(dataDir, "backups", time.Now().Format("2006-01-02T15-04-05"))
	if err := cli.EnsureDir(filepath.Join(dataDir, "backups"), 0o700); err != nil {
		return Report{}, "", err
	}

	resolve, err := newResolver(ios, onConflict)
	if err != nil {
		return Report{}, "", err
	}
	report, err := Apply(Tree{Source: scaffold.HomeDir(repo), Target: home, Copy: scaffold.CopyDirs}, resolve, backupDir)
	if err != nil {
		return report, backupDir, err
	}

	// Legacy files like ~/.gitconfig and ~/.zshrc shadow the rendered XDG
	// configuration without occupying any link site, so the conflict flow
	// above never reaches them; retire them into the same backup set.
	if err := retireLegacy(home, backupDir, &report); err != nil {
		return report, backupDir, err
	}

	configDir, err := cli.ConfigDir()
	if err != nil {
		return report, backupDir, err
	}
	if err := linkProfiles(repo, configDir, resolve, backupDir, &report); err != nil {
		return report, backupDir, err
	}

	// Per-profile files link through the active-profile symlink, so
	// activating a different profile swaps them without relinking.
	ops, err := scaffold.Plan(a)
	if err != nil {
		return report, backupDir, err
	}
	for _, op := range ops {
		rel, ok := strings.CutPrefix(op.Dst, "home/")
		if !op.PerProfile || !ok {
			continue // env.zsh and git.gitconfig are reached by path, not linked
		}
		site := filepath.Join(home, rel)
		dest := filepath.Join(configDir, "active-profile", op.Dst)
		if err := ApplyFile(site, dest, resolve, backupDir, &report); err != nil {
			return report, backupDir, err
		}
	}
	return report, backupDir, err
}

// linkProfiles links every profile the repository carries to its live
// location under the config directory, so the active-profile symlink always
// resolves into the repository and machines of the same class share one
// profile. A real directory in the way — state from before profiles lived in
// the repository — is a conflict whose resolution can only back it up: its
// content is stale machine-local state, never fresher than the repository's.
func linkProfiles(repo, configDir string, resolve Resolver, backupRoot string, rep *Report) error {
	entries, err := os.ReadDir(scaffold.ProfilesDir(repo))
	if err != nil {
		return fmt.Errorf("read repository profiles: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() || profile.ValidateName(e.Name()) != nil {
			continue
		}
		site := profile.Dir(configDir, e.Name())
		dest := filepath.Join(scaffold.ProfilesDir(repo), e.Name())
		if err := ApplyFile(site, dest, resolve, backupRoot, rep); err != nil {
			return err
		}
	}
	return nil
}

// newResolver maps --on-conflict to a Resolver; with the flag unset on an
// interactive run, each conflict is asked about, with sticky "all" answers.
// Generated destinations never offer adoption — the destination was just
// rendered from the profile's answers, and adopting a stale copy over it
// would revert the render.
func newResolver(ios cli.IOStreams, onConflict string) (Resolver, error) {
	fixed := map[string]Resolution{
		"backup": ResBackup, "adopt": ResAdopt,
		"skip": ResSkip, "fail": ResFail,
	}
	if onConflict != "" {
		res, ok := fixed[onConflict]
		if !ok {
			return nil, fmt.Errorf("unknown --on-conflict value %q", onConflict)
		}
		return func(Conflict) (Resolution, error) { return res, nil }, nil
	}
	if !ios.IsInteractive() {
		return func(Conflict) (Resolution, error) { return ResBackup, nil }, nil
	}

	var sticky *Resolution
	return func(c Conflict) (Resolution, error) {
		if sticky != nil {
			return *sticky, nil
		}
		options := []tui.Option{{Label: "back it up, then link", Value: "backup"}}
		if !c.Generated {
			options = append(options, tui.Option{Label: "adopt it into the repository, then link", Value: "adopt"})
		}
		options = append(options,
			tui.Option{Label: "skip it", Value: "skip"},
			tui.Option{Label: "back up this and all remaining", Value: "backup-all"},
		)
		if !c.Generated {
			options = append(options, tui.Option{Label: "adopt this and all remaining", Value: "adopt-all"})
		}
		options = append(options, tui.Option{Label: "abort", Value: "fail"})
		picked, err := tui.FuzzySelect(ios, fmt.Sprintf("%s exists — what should happen?", c.Site), options)
		if errors.Is(err, tui.ErrAborted) {
			return ResFail, nil
		}
		if err != nil {
			return ResFail, err
		}
		if all, ok := strings.CutSuffix(picked, "-all"); ok {
			res := fixed[all]
			sticky = &res
			return res, nil
		}
		return fixed[picked], nil
	}, nil
}

// PruneSites removes the live symlinks a render prune orphaned: for each
// home-relative pruned path, the site is removed only when it is a symlink
// that no longer resolves — real files and working links are never touched.
func PruneSites(ios cli.IOStreams, home string, pruned []string) {
	for _, rel := range pruned {
		site := filepath.Join(home, rel)
		info, err := os.Lstat(site)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if _, err := os.Stat(site); err == nil {
			continue // resolves somewhere live; not this prune's leftover
		}
		if err := os.Remove(site); err == nil {
			tui.Infof(ios, "Removed dangling link %s", site)
		}
	}
}

// Summarize reports what linking changed and where backups went.
func Summarize(ios cli.IOStreams, rep Report, backupDir string) {
	tui.Successf(ios, "Linked %d, replaced %d stale links, %d already correct",
		len(rep.Linked), len(rep.Replaced), rep.OK)
	if len(rep.Copied) > 0 {
		tui.Infof(ios, "Mirrored %d copy-deployed directories: %s", len(rep.Copied), strings.Join(rep.Copied, ", "))
	}
	if len(rep.Adopted) > 0 {
		tui.Infof(ios, "Adopted %d existing files into the repository — review with git diff", len(rep.Adopted))
	}
	if len(rep.Skipped) > 0 {
		tui.Warnf(ios, "Skipped %d conflicting files: %s", len(rep.Skipped), strings.Join(rep.Skipped, ", "))
	}
	if len(rep.Retired) > 0 {
		tui.Infof(ios, "Retired legacy files shadowing the rendered config: %s", strings.Join(rep.Retired, ", "))
	}
	if len(rep.Backed) > 0 || len(rep.Retired) > 0 {
		tui.Infof(ios, "Backed up %d files under %s (restore with dotty dotfiles restore)",
			len(rep.Backed)+len(rep.Retired), backupDir)
	}
}
