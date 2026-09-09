// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package fonts

import "path/filepath"

// Dir returns the per-user font directory fonts install into.
func Dir(home string) string {
	return filepath.Join(home, "Library", "Fonts")
}
