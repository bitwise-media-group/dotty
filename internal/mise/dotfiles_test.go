// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDotfilesRoot(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("HOME", filepath.Dir(repo))
	dir := filepath.Join(repo, "profiles", "work", "mise")
	if err := EnsureConfig(dir); err != nil {
		t.Fatal(err)
	}
	client := New(&fakeRunner{}, "mise", dir)
	for range 2 {
		if err := client.DotfilesRoot(filepath.Join(repo, "home")); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := `root = "~/` + filepath.Base(repo) + `/home"`
	if strings.Count(string(data), `[settings.dotfiles]`) != 1 || !strings.Contains(string(data), wantRoot) {
		t.Fatalf("capture root config:\n%s", data)
	}
}

func TestDotfilesPassthrough(t *testing.T) {
	r := &fakeRunner{}
	c := New(r, "mise", t.TempDir())
	if err := c.Dotfiles(context.Background(), "track", "--dry-run", "~/a file"); err != nil {
		t.Fatal(err)
	}
	want := "interactive dot --cd " + c.Dir + " track --dry-run ~/a file"
	if got := r.argvs(t, "mise", c.Dir); len(got) != 1 || got[0] != want {
		t.Fatalf("calls = %v", got)
	}
}
