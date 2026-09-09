// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

//go:build !darwin

package fonts

import (
	"os"
	"path/filepath"
)

// Dir returns the per-user font directory fonts install into:
// $XDG_DATA_HOME/fonts, or ~/.local/share/fonts when the variable is unset
// or not absolute as the XDG spec requires. Fontconfig scans it by default
// and notices new files by directory mtime, so no fc-cache run is needed.
func Dir(home string) string {
	if base := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(base) {
		return filepath.Join(base, "fonts")
	}
	return filepath.Join(home, ".local", "share", "fonts")
}
