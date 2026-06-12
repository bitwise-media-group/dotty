// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDir(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want func(home string) string
	}{
		{
			name: "absolute env value wins",
			env:  "/custom/config",
			want: func(string) string { return "/custom/config/dotty" },
		},
		{
			name: "empty env falls back to ~/.config",
			env:  "",
			want: func(home string) string { return filepath.Join(home, ".config", "dotty") },
		},
		{
			name: "relative env value is ignored per the XDG spec",
			env:  "relative/path",
			want: func(home string) string { return filepath.Join(home, ".config", "dotty") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tt.env)
			got, err := ConfigDir()
			if err != nil {
				t.Fatalf("ConfigDir() error: %v", err)
			}
			home, _ := os.UserHomeDir()
			if want := tt.want(home); got != want {
				t.Errorf("ConfigDir() = %q, want %q", got, want)
			}
		})
	}
}

func TestDataDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")
	got, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error: %v", err)
	}
	if got != "/custom/data/dotty" {
		t.Errorf("DataDir() = %q, want /custom/data/dotty", got)
	}

	t.Setenv("XDG_DATA_HOME", "")
	got, err = DataDir()
	if err != nil {
		t.Fatalf("DataDir() error: %v", err)
	}
	home, _ := os.UserHomeDir()
	if want := filepath.Join(home, ".local", "share", "dotty"); got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
}

func TestEnsureDir(t *testing.T) {
	t.Run("creates nested directories with perm", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "a", "b")
		if err := EnsureDir(dir, 0o700); err != nil {
			t.Fatalf("EnsureDir() error: %v", err)
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("perm = %o, want 700", perm)
		}
	})

	t.Run("tightens an existing looser directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "loose")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := EnsureDir(dir, 0o700); err != nil {
			t.Fatalf("EnsureDir() error: %v", err)
		}
		info, _ := os.Stat(dir)
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("perm = %o, want 700", perm)
		}
	})
}

func TestAtomicWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.json")

	if err := AtomicWriteFile(path, []byte(`{"v":1}`), 0o600); err != nil {
		t.Fatalf("AtomicWriteFile() error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != `{"v":1}` {
		t.Errorf("content = %q", got)
	}
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}

	// Overwrite must replace content and leave no temp droppings.
	if err := AtomicWriteFile(path, []byte(`{"v":2}`), 0o600); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want 1", len(entries))
	}
}
