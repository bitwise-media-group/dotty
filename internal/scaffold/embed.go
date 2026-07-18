// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import "embed"

// templateFS is the dotfiles template. The all: prefix is load-bearing —
// nearly every tree in the template is dot-named, and a plain go:embed would
// silently drop them.
//
//go:embed all:template
var templateFS embed.FS
