// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package profile manages dotty's system profiles: per-machine configuration
// sets (mise packages, per-profile renders, answers) that live under
// $XDG_CONFIG_HOME/dotty/<name> and travel across machines via a public
// dotfiles repository. The active profile is the active-profile symlink
// beside them.
package profile
