// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// TestPickRepo pins the non-interactive selection paths: a query narrowing to
// one repository resolves without prompting, and zero matches fail with
// guidance. The interactive picklist itself is huh's concern, not ours.
func TestPickRepo(t *testing.T) {
	root := t.TempDir()
	for _, repo := range []string{"org/dotty", "org/dotfiles", "acme/webapp"} {
		if err := os.MkdirAll(filepath.Join(root, repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("REPOS_DIR", root)

	tests := []struct {
		name    string
		query   string
		want    string // resolved path relative to root; empty means an error
		wantErr string
	}{
		{name: "unique match auto-selects", query: "webapp", want: "acme/webapp"},
		{name: "match is case-insensitive", query: "WEBAPP", want: "acme/webapp"},
		{name: "no match fails with the query", query: "nope", wantErr: `no repository matching "nope"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pickRepo(cli.System(), tt.query)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("pickRepo(%q) error = %v, want %q", tt.query, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("pickRepo(%q): %v", tt.query, err)
			}
			if want := filepath.Join(root, tt.want); got != want {
				t.Errorf("pickRepo(%q) = %q, want %q", tt.query, got, want)
			}
		})
	}
}

// TestPickRepoDotQuery resolves "." to the repository enclosing the working
// directory without consulting $REPOS_DIR.
func TestPickRepoDotQuery(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REPOS_DIR", t.TempDir()) // empty; must not be searched
	t.Chdir(repo)

	got, err := pickRepo(cli.System(), ".")
	if err != nil {
		t.Fatalf("pickRepo(.): %v", err)
	}
	// TempDir may sit behind a symlink (macOS /var -> /private/var); compare resolved paths.
	wantResolved, _ := filepath.EvalSymlinks(repo)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != wantResolved {
		t.Errorf("pickRepo(.) = %q, want %q", got, repo)
	}
}
