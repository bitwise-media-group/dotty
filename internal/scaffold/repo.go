// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitwise-media-group/dotty/internal/brewfile"
	"github.com/bitwise-media-group/dotty/internal/cli"
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

// IsRepo reports whether dir is a dotty-made dotfiles repository: the
// .dotty-version marker (which records the release that rendered the repo,
// for future upgrades), or a structural profile layout — top-level profiles/
// or the legacy in-tree location — for repositories rendered before the
// marker existed.
func IsRepo(dir string) bool {
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
// Brewfile). Only the active-profile symlink, the machine's choice of
// profile, stays local. A repository in the legacy layout is migrated first,
// and renders the plan no longer produces are pruned from the profile — the
// migration preserves paths as-is, so a template relocation would otherwise
// leave a stale render at the old destination. It returns the pruned paths
// relative to the profile's home/ tree.
func RenderRepository(ctx context.Context, ios cli.IOStreams, r brewfile.Runner,
	answers Answers, repo, home string) ([]string, error) {
	if err := MigrateLayout(ios, repo); err != nil {
		return nil, err
	}

	repoProfileDir := profile.Dir(ProfilesDir(repo), answers.ProfileName)
	answers = withMetadata(answers, repoProfileDir)
	if err := cli.EnsureDir(repoProfileDir, 0o755); err != nil {
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

	return pruned, composeProfileBrewfile(ctx, ios, r, repoProfileDir, answers)
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

// composeProfileBrewfile writes the composed Brewfile into the repository's
// profile directory, seeding from brew bundle dump first when asked. Written
// before Activate on purpose: Activate dumps only when none exists.
func composeProfileBrewfile(ctx context.Context, ios cli.IOStreams, r brewfile.Runner,
	repoProfileDir string, a Answers) error {
	composed, err := ComposeBrewfile(a)
	if err != nil {
		return err
	}
	brewPath := profile.BrewfilePath(repoProfileDir)

	// Seeding is a convenience — a broken brew must not fail the init, it
	// just means the Brewfile starts from the template alone.
	if a.DumpBrews {
		if err := brewfile.Dump(ctx, r, brewPath, false, true); err != nil {
			tui.Warnf(ios, "Could not seed from the installed packages (dotty brewfile dump retries later): %v", err)
		} else if existing, err := os.ReadFile(brewPath); err == nil {
			composed = MergeBrewfile(existing, composed)
		}
	}
	return cli.AtomicWriteFile(brewPath, composed, 0o644)
}
