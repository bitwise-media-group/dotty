// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package macos

import (
	"context"
	"fmt"
	"strings"
)

// Runner runs the system tools the settings go through. RunInteractive
// covers sudo, which needs the terminal for its password prompt.
type Runner interface {
	RunInteractive(ctx context.Context, name string, args ...string) error
}

// Group is one selectable set of defaults. The settings themselves live in
// the scaffold's mise fragment for the group (template/mise.d/macos-<id>.toml),
// which mise applies; NeedsLogout marks the groups macOS only reads at login.
type Group struct {
	ID          string
	Label       string
	NeedsLogout bool
}

// Groups are the curated defaults, in menu order. IDs are what Answers
// persists and name each group's mise fragment, so renaming one orphans
// saved selections.
var Groups = []Group{
	{
		ID: "keyboard", Label: "keyboard: key repeat instead of the accent popup, auto-capitalise and period",
		NeedsLogout: true,
	},
	{ID: "input-sources", Label: "input sources: ABC layout, input menu hidden", NeedsLogout: true},
	{ID: "menu-bar", Label: "menu bar: always visible, except in full screen"},
	{
		ID: "trackpad", Label: "trackpad: tap to click, light click, three-finger drag, four-finger swipes, " +
			"max tracking speed",
		NeedsLogout: true,
	},
	{
		ID: "mouse", Label: "mouse: Magic Mouse primary click only, swipe between full-screen apps, " +
			"double-tap for Mission Control",
		NeedsLogout: true,
	},
	{ID: "finder", Label: "finder: clean desktop, path/status bars, no extension warnings"},
	{ID: "screenshots", Label: "screenshots: save as png"},
	{ID: "software-update", Label: "software update: check daily"},
	{ID: "spaces", Label: "spaces: per-display, never rearranged"},
	{ID: "dock", Label: "dock: current layout and sizing, minimize to application"},
	{ID: "animations", Label: "animations: disable window animations"},
	{ID: "gpg-keychain", Label: "gpg: cache PINs in the macOS keychain"},
	{ID: "default-terminal", Label: "default terminal: Ghostty opens shell scripts and executables"},
}

// EnforcePIV requires smart-card login system-wide. sudo prompts on the
// terminal; the caller confirms first — this can lock a machine without an
// enrolled card out. It stays in dotty because it writes
// /Library/Preferences, which mise's user defaults cannot.
func EnforcePIV(ctx context.Context, r Runner) error {
	for _, args := range [][]string{
		{"defaults", "write", "/Library/Preferences/com.apple.security.smartcard", "enforceSmartCard", "-bool", "true"},
		{"defaults", "write", "/Library/Preferences/com.apple.security.smartcard", "allowUnmappedUsers", "-int", "1"},
	} {
		if err := r.RunInteractive(ctx, "sudo", args...); err != nil {
			return fmt.Errorf("sudo %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}
