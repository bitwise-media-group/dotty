// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/mise"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// HomeDir returns the repository's $HOME-shaped tree — the entries linked
// over the home directory.
func HomeDir(repo string) string { return filepath.Join(repo, "home") }

// ProfilesDir returns the directory the repository's profiles live in.
func ProfilesDir(repo string) string { return filepath.Join(repo, "profiles") }

// legacyProfilesDir is where profiles lived before they lifted to the top
// level: inside the linked tree, under .config/dotty.
func legacyProfilesDir(repo string) string {
	return filepath.Join(repo, "stow", ".config", "dotty")
}

// PrivateMarker identifies an encrypted private dotfiles repository (see the
// privdot package). It is deliberately not .dotty-version: a private repo has
// profiles/ too, and IsRepo must not claim it — dotty dotfiles verbs would
// misroute onto a repository with no home tree.
const PrivateMarker = ".dotty-private"

// IsRepo reports whether dir is a dotty-made dotfiles repository: the
// .dotty-version marker (which records the release that rendered the repo,
// for future upgrades), or a structural profile layout — top-level profiles/
// or the legacy in-tree location — for repositories rendered before the
// marker existed. A private repository (PrivateMarker) is never a dotfiles
// repository, whatever its shape.
func IsRepo(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, PrivateMarker)); err == nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, ".dotty-version")); err == nil {
		return true
	}
	for _, profiles := range []string{ProfilesDir(dir), legacyProfilesDir(dir)} {
		if info, err := os.Stat(profiles); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// ListRepoProfiles returns the names of the profiles a repository carries,
// looking in the current layout and the legacy one.
func ListRepoProfiles(repo string) []string {
	var names []string
	for _, dir := range []string{ProfilesDir(repo), legacyProfilesDir(repo)} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() && profile.ValidateName(e.Name()) == nil {
				names = append(names, e.Name())
			}
		}
	}
	return names
}

// LoadRepoAnswers loads a named profile's answers from a repository, looking
// in the current layout and the legacy one, along with the set of questions
// the stored document answers.
func LoadRepoAnswers(repo, name string) (Answers, AnswerKeys, error) {
	a, keys, err := LoadAnswersWithKeys(profile.Dir(ProfilesDir(repo), name))
	if errors.Is(err, fs.ErrNotExist) {
		return LoadAnswersWithKeys(profile.Dir(legacyProfilesDir(repo), name))
	}
	return a, keys, err
}

// EnclosingRepo walks up from the working directory looking for a dotty-made
// dotfiles repository; "" when there is none.
func EnclosingRepo() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if IsRepo(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// RenderRepository renders the template into the repository, including the
// profile: the profile lives at profiles/<name> so a machine class —
// personal, work — shares it across machines (answers, per-profile renders,
// the mise package directory). Only the active-profile symlink, the
// machine's choice of profile, stays local. A repository in the legacy
// layout is migrated first, and renders the plan no longer produces are
// pruned from the profile — the migration preserves paths as-is, so a
// template relocation would otherwise leave a stale render at the old
// destination. It returns the pruned paths relative to the profile
// directory.
func RenderRepository(ctx context.Context, ios cli.IOStreams, r mise.Runner,
	answers Answers, repo, home string) ([]string, error) {
	if err := MigrateLayout(ios, repo); err != nil {
		return nil, err
	}

	repoProfileDir := profile.Dir(ProfilesDir(repo), answers.ProfileName)
	answers = withMetadata(answers, repoProfileDir)
	if err := cli.EnsureDir(repoProfileDir, 0o755); err != nil {
		return nil, err
	}
	// Seed before rendering: the template's config.toml is a keep-existing
	// render, so a seeded file wins over the empty default.
	if err := seedMiseConfig(ios, mise.Dir(repoProfileDir)); err != nil {
		return nil, err
	}

	ops, err := Plan(answers)
	if err != nil {
		return nil, err
	}
	vars := NewVars(answers, home, repo)
	if err := Render(ops, repo, repoProfileDir, vars); err != nil {
		return nil, err
	}
	pruned, err := PrunePerProfile(repoProfileDir, ops)
	if err != nil {
		return nil, err
	}
	if err := SaveAnswers(repoProfileDir, answers); err != nil {
		return nil, err
	}
	tui.Successf(ios, "Rendered %d files into %s (profile %s inside it)", len(ops), repo, answers.ProfileName)
	if len(pruned) > 0 {
		tui.Infof(ios, "Pruned %d obsolete profile renders: %s", len(pruned), strings.Join(pruned, ", "))
	}

	return pruned, composeProfilePackages(ctx, ios, r, repoProfileDir, answers, home)
}

// withMetadata completes the profile metadata that shares profile.json with
// the answers: an existing profile's description and creation time are
// preserved when the answers do not already carry them, and a brand-new
// profile is stamped as created now.
func withMetadata(a Answers, profileDir string) Answers {
	if a.Description != "" && !a.CreatedAt.IsZero() {
		return a
	}
	if p, err := profile.Load(filepath.Dir(profileDir), a.ProfileName); err == nil {
		if a.Description == "" {
			a.Description = p.Description
		}
		if a.CreatedAt.IsZero() {
			a.CreatedAt = p.CreatedAt
		}
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if a.Description == "" {
		a.Description = "created by dotty init"
	}
	return a
}

// seedMiseConfig gives a profile that has no mise config.toml yet the one
// the machine already runs on: when ~/.config/mise is still a real
// directory with a config.toml (from before dotty owned it), its
// config.toml and mise.lock are copied into the profile, with the settings
// dotty relies on — the lockfile, cask adoption — added if missing. The linker backs the real directory up afterwards, so
// nothing is lost either way. A profile that already has a config.toml is
// left alone.
func seedMiseConfig(ios cli.IOStreams, miseDir string) error {
	if _, err := os.Stat(mise.ConfigPath(miseDir)); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", mise.ConfigPath(miseDir), err)
	}
	configDir, err := cli.ConfigDir()
	if err != nil {
		return err
	}
	live := filepath.Join(filepath.Dir(configDir), "mise")
	if info, err := os.Lstat(live); err != nil || !info.IsDir() {
		return nil // no live config, or already the profile link
	}
	data, err := os.ReadFile(mise.ConfigPath(live))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", mise.ConfigPath(live), err)
	}
	data, _ = mise.MergeEntries(data, "settings", []string{"lockfile = true"}, "dotty: lock every tool")
	data, _ = mise.MergeEntries(data, "bootstrap.brew", []string{"adopt = true"}, "dotty: keep the casks already installed")
	if err := cli.EnsureDir(miseDir, 0o755); err != nil {
		return err
	}
	if err := cli.AtomicWriteFile(mise.ConfigPath(miseDir), data, 0o644); err != nil {
		return err
	}
	if lock, err := os.ReadFile(mise.LockPath(live)); err == nil {
		if err := cli.AtomicWriteFile(mise.LockPath(miseDir), lock, 0o644); err != nil {
			return err
		}
	}
	tui.Successf(ios, "Seeded %s from %s", mise.ConfigPath(miseDir), live)
	return nil
}

// composeProfilePackages fills in the user-owned side of the profile's
// packages after the fragments are rendered: a Brewfile left over from
// before packages moved to mise is converted into config.toml entries, and
// when asked the machine's installed Homebrew formulae are imported. Both
// merge — an existing config.toml is user-owned, its lines are preserved
// verbatim and only genuinely new entries are appended under a header
// comment — and skip what the fragments already declare.
func composeProfilePackages(ctx context.Context, ios cli.IOStreams, r mise.Runner,
	repoProfileDir string, a Answers, home string) error {
	miseDir := mise.Dir(repoProfileDir)
	if err := mise.EnsureConfig(miseDir); err != nil {
		return err
	}
	if err := convertProfileBrewfile(ios, repoProfileDir); err != nil {
		return err
	}
	if !a.ImportPackages {
		return nil
	}
	// Importing is a convenience — a missing mise or brew must not fail
	// the init, it just means the profile starts without the installed
	// packages.
	bin, err := mise.Lookup(exec.LookPath, home)
	if err != nil {
		tui.Warnf(ios, "Could not import the installed packages (dotty packages import retries later): %v", err)
		return nil
	}
	entries, err := mise.ImportToScratch(ctx, r, bin, miseDir, false)
	if err != nil {
		tui.Warnf(ios, "Could not import the installed packages (dotty packages import retries later): %v", err)
		return nil
	}
	added, err := mise.MergeImported(miseDir, entries)
	if err != nil {
		return err
	}
	if added > 0 {
		tui.Successf(ios, "Imported %d installed packages into %s", added, mise.ConfigPath(miseDir))
	}
	return nil
}

// convertProfileBrewfile merges a legacy profile Brewfile into the mise
// config.toml as tools and bootstrap packages. The Brewfile stays where it
// is — the conversion is best-effort and the file is the user's to delete
// once the result checks out; a re-run finds nothing new and stays quiet.
func convertProfileBrewfile(ios cli.IOStreams, repoProfileDir string) error {
	brewPath := filepath.Join(repoProfileDir, "Brewfile")
	data, err := os.ReadFile(brewPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", brewPath, err)
	}
	miseDir := mise.Dir(repoProfileDir)
	conv := mise.ConvertBrewfile(data)
	added, err := mise.MergeConversion(miseDir, conv)
	if err != nil || added == 0 {
		return err
	}
	for _, w := range conv.Warnings {
		tui.Warnf(ios, "Brewfile: %s", w)
	}
	tui.Successf(ios, "Converted %d Brewfile entries into %s", added, mise.ConfigPath(miseDir))
	tui.Infof(ios, "%s is no longer read; delete it once the packages look right", brewPath)
	return nil
}
