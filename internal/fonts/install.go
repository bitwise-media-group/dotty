// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package fonts

import (
	"context"
	"net/http"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// Install downloads the lobe-icons glyph font into the user font directory.
// Never fatal: a font is not worth failing init over.
func Install(ctx context.Context, ios cli.IOStreams, home string) {
	client := &http.Client{Timeout: 30 * time.Second}
	installed, err := InstallLobeIcons(ctx, client, LobeIconsURL, Dir(home))
	if err != nil {
		tui.Warnf(ios, "Could not install lobe-icons font (rerun init to retry): %v", err)
		return
	}
	if installed {
		tui.Successf(ios, "Installed lobe-icons.ttf for the AI-brand glyphs in dotty tmux new")
	}
}
