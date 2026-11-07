// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/linker"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// linkCodexSystemConfig points Codex's system config layer at the active
// profile's settings, warning instead of failing — the dotfiles are already
// linked, and a declined sudo is not worth unwinding them over. A variable so
// tests never reach sudo or /etc.
var linkCodexSystemConfig = func(ctx context.Context, ios cli.IOStreams, home, configDir string) {
	err := linker.LinkCodexSystemConfig(ctx, ios, newRunner(ios), linker.CodexSystemConfig, home, configDir)
	if err != nil {
		tui.Warnf(ios, "Codex system config not linked: %v (retry with dotty dotfiles link)", err)
	}
}
