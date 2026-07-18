// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// MigrateLayout restructures a repository from the legacy layout to the
// current one, in place:
//
//   - profiles move from stow/.config/dotty/<name> to profiles/<name>
//   - each profile's render/ ceremony dissolves — render/stow becomes the
//     profile's home/ tree and the loose renders (env.zsh, git.gitconfig,
//     worktrees.gitconfig) move to the profile root
//   - each profile's dotty.json merges into its profile.json
//   - the stow/ tree is renamed to home/
//
// A repository already in the current layout is left untouched. The moves
// are plain renames so `git status` shows them for review; $HOME symlinks
// into old paths are healed by the relink that follows a re-render.
func MigrateLayout(ios cli.IOStreams, repo string) error {
	legacy := legacyProfilesDir(repo)
	if info, err := os.Stat(legacy); err != nil || !info.IsDir() {
		return migrateTree(ios, repo) // profiles already lifted; tree may still need renaming
	}

	entries, err := os.ReadDir(legacy)
	if err != nil {
		return fmt.Errorf("migrate layout: %w", err)
	}
	if err := cli.EnsureDir(ProfilesDir(repo), 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		src := filepath.Join(legacy, e.Name())
		dst := filepath.Join(ProfilesDir(repo), e.Name())
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("migrate layout: both %s and %s exist; remove one", src, dst)
		}
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("migrate profile %s: %w", e.Name(), err)
		}
		if err := migrateProfile(dst); err != nil {
			return err
		}
		tui.Successf(ios, "Migrated profile %s to %s", e.Name(), dst)
	}
	if err := os.Remove(legacy); err != nil {
		return fmt.Errorf("migrate layout: remove %s: %w", legacy, err)
	}
	// .config/dotty was the only reason .config existed in some repos; a
	// now-empty parent is just noise. Non-empty parents stay.
	_ = os.Remove(filepath.Join(repo, "stow", ".config"))

	return migrateTree(ios, repo)
}

// migrateTree renames the repository's stow/ tree to home/.
func migrateTree(ios cli.IOStreams, repo string) error {
	legacy := filepath.Join(repo, "stow")
	if info, err := os.Lstat(legacy); err != nil || !info.IsDir() {
		return nil
	}
	if _, err := os.Stat(HomeDir(repo)); err == nil {
		return fmt.Errorf("migrate layout: both %s and %s exist; remove one", legacy, HomeDir(repo))
	}
	if err := os.Rename(legacy, HomeDir(repo)); err != nil {
		return fmt.Errorf("migrate layout: rename %s: %w", legacy, err)
	}
	tui.Successf(ios, "Renamed %s to %s", legacy, HomeDir(repo))
	return nil
}

// migrateProfile modernizes one moved profile directory: the render/
// indirection dissolves and dotty.json merges into profile.json.
func migrateProfile(dir string) error {
	render := filepath.Join(dir, "render")
	if _, err := os.Stat(render); err == nil {
		if _, err := os.Stat(filepath.Join(render, "stow")); err == nil {
			if err := os.Rename(filepath.Join(render, "stow"), filepath.Join(dir, "home")); err != nil {
				return fmt.Errorf("migrate %s: %w", dir, err)
			}
		}
		loose, err := os.ReadDir(render)
		if err != nil {
			return fmt.Errorf("migrate %s: %w", dir, err)
		}
		for _, e := range loose {
			if err := os.Rename(filepath.Join(render, e.Name()), filepath.Join(dir, e.Name())); err != nil {
				return fmt.Errorf("migrate %s: %w", dir, err)
			}
		}
		if err := os.Remove(render); err != nil {
			return fmt.Errorf("migrate %s: remove render dir: %w", dir, err)
		}
	}
	return mergeProfileDocuments(dir)
}

// mergeProfileDocuments folds a legacy dotty.json (answers) into profile.json
// (metadata): the answers win key-for-key, metadata-only keys survive, and
// dotty.json disappears.
func mergeProfileDocuments(dir string) error {
	answersData, err := os.ReadFile(filepath.Join(dir, legacyAnswersFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("merge %s: %w", legacyAnswersFile, err)
	}

	merged := map[string]json.RawMessage{}
	if metaData, err := os.ReadFile(filepath.Join(dir, AnswersFile)); err == nil {
		if err := json.Unmarshal(metaData, &merged); err != nil {
			return fmt.Errorf("parse %s: %w", filepath.Join(dir, AnswersFile), err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var answers map[string]json.RawMessage
	if err := json.Unmarshal(answersData, &answers); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Join(dir, legacyAnswersFile), err)
	}
	maps.Copy(merged, answers)

	data, err := json.MarshalIndent(merged, "", "\t")
	if err != nil {
		return fmt.Errorf("encode %s: %w", AnswersFile, err)
	}
	if err := cli.AtomicWriteFile(filepath.Join(dir, AnswersFile), append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, legacyAnswersFile))
}
