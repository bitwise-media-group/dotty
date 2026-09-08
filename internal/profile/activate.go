// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package profile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Activate points the active-profile symlink at the named profile and returns
// the profile's directory. The swap is atomic — a temp symlink renamed over
// the old one — so the link never dangles mid-switch. The symlink target is
// the bare profile name (relative), which survives a home-directory move and
// reads cleanly in a dotfiles repository.
//
// Everything reached through the link — the per-profile renders, the mise
// package directory behind ~/.config/mise — swaps with it; converging the
// machine's packages on the new profile is `dotty packages sync`'s job.
func Activate(configDir, name string) (string, error) {
	if !Exists(configDir, name) {
		return "", fmt.Errorf("profile %q: %w", name, ErrNotFound)
	}
	tmp := filepath.Join(configDir, ".active-profile.tmp")
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("clear stale temp symlink: %w", err)
	}
	if err := os.Symlink(name, tmp); err != nil {
		return "", fmt.Errorf("create temp symlink: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(configDir, activeLink)); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("swap active-profile symlink: %w", err)
	}
	return Dir(configDir, name), nil
}
