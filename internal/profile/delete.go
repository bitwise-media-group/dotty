// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var (
	// ErrLastProfile reports a delete that would leave the machine with no
	// profile at all.
	ErrLastProfile = errors.New("cannot delete the last profile")
	// ErrActiveProfile reports a delete of the profile the active-profile
	// symlink points at; activate a different one first.
	ErrActiveProfile = errors.New("profile is active")
)

// Locate reports where a profile lives: its entry under the config dir and,
// when that entry is a symlink, the directory it resolves to. dotty links
// every profile the dotfiles repository carries into the config dir, so for a
// repository-backed profile the two differ and the second is the one holding
// the Brewfile, the renders, and the answers. backing is empty for a profile
// that is a real directory — one made by hand, or by an older dotty on a
// machine with no repository yet.
func Locate(configDir, name string) (site, backing string, err error) {
	site = Dir(configDir, name)
	info, err := os.Lstat(site)
	if errors.Is(err, fs.ErrNotExist) {
		return "", "", fmt.Errorf("profile %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return "", "", fmt.Errorf("stat %s: %w", site, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return site, "", nil
	}
	target, err := os.Readlink(site)
	if err != nil {
		return "", "", fmt.Errorf("read %s symlink: %w", site, err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(configDir, target)
	}
	return site, target, nil
}

// Delete removes a profile: the entry under the config dir and, when that
// entry links into the dotfiles repository, the directory behind it. Removing
// only the link would delete nothing — the profile's content lives in the
// repository, and the next `dotty dotfiles link` would restore the link from
// it.
//
// Two invariants guard the delete. The last profile cannot go: a machine is
// always of some class, and every picklist that names profiles would have
// nothing left to offer. Neither can the active one, so the active-profile
// symlink — which the shared git and zsh config reach through — never
// dangles; activate a replacement first.
func Delete(configDir, name string) error {
	if !Exists(configDir, name) {
		return fmt.Errorf("profile %q: %w", name, ErrNotFound)
	}
	profiles, err := List(configDir)
	if err != nil {
		return err
	}
	if len(profiles) <= 1 {
		return fmt.Errorf("profile %q: %w", name, ErrLastProfile)
	}
	active, err := ActiveName(configDir)
	if err != nil && !errors.Is(err, ErrNoActiveProfile) {
		return err
	}
	if active == name {
		return fmt.Errorf("profile %q: %w", name, ErrActiveProfile)
	}

	site, backing, err := Locate(configDir, name)
	if err != nil {
		return err
	}
	// The backing directory goes first. A failure part-way then leaves a
	// dangling link, which reads as no-such-profile and has nothing left to
	// relink from; the reverse order would leave repository content behind
	// for the next `dotty dotfiles link` to silently resurrect.
	if backing != "" {
		if err := os.RemoveAll(backing); err != nil {
			return fmt.Errorf("remove profile directory %s: %w", backing, err)
		}
	}
	if err := os.RemoveAll(site); err != nil {
		return fmt.Errorf("remove %s: %w", site, err)
	}
	return nil
}
