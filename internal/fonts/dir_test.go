// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package fonts

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDir(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(t.TempDir(), "xdg")
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "default", env: "", want: filepath.Join(home, ".local", "share", "fonts")},
		{name: "xdg data home", env: xdg, want: filepath.Join(xdg, "fonts")},
		{name: "relative xdg ignored", env: "relative/share", want: filepath.Join(home, ".local", "share", "fonts")},
	}
	if runtime.GOOS == "darwin" {
		for i := range tests {
			tests[i].want = filepath.Join(home, "Library", "Fonts")
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", tt.env)
			if got := Dir(home); got != tt.want {
				t.Errorf("Dir(%q) = %q, want %q", home, got, tt.want)
			}
		})
	}
}
