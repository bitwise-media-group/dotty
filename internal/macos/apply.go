// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package macos

import (
	"context"
	"runtime"
	"slices"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// FinishSelections runs what is left of the macOS selections once mise has
// applied the defaults groups: a note when a group only takes effect after
// logging out, and PIV enforcement. Best-effort — init has already succeeded
// by the time it runs, and a declined sudo is not worth unwinding it over.
// A no-op off darwin.
func FinishSelections(ctx context.Context, ios cli.IOStreams, r Runner, groups []string, piv bool) {
	if runtime.GOOS != "darwin" {
		return
	}
	if NeedsLogout(groups) {
		tui.Infof(ios, "Keyboard, input-source, trackpad and mouse defaults take effect after you log out")
	}
	if piv {
		tui.Infof(ios, "Enforcing smart-card login needs sudo")
		if err := EnforcePIV(ctx, r); err != nil {
			tui.Warnf(ios, "Could not enforce smart-card login: %v", err)
		} else {
			tui.Successf(ios, "Smart-card (PIV) login required system-wide")
		}
	}
}

// NeedsLogout reports whether any of the selected groups is one macOS only
// reads at login.
func NeedsLogout(groups []string) bool {
	return slices.ContainsFunc(Groups, func(g Group) bool {
		return g.NeedsLogout && slices.Contains(groups, g.ID)
	})
}
