// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsRepoExcludesPrivateRepositories(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "profiles", "personal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(dir) {
		t.Fatal("a top-level profiles/ directory should read as a dotfiles repository")
	}
	if err := os.WriteFile(filepath.Join(dir, PrivateMarker), []byte("{\"version\":\"v0\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsRepo(dir) {
		t.Error("the private marker must exclude a repository, whatever its shape")
	}
	if err := os.WriteFile(filepath.Join(dir, ".dotty-version"), []byte("v0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsRepo(dir) {
		t.Error("the private marker must win even against .dotty-version")
	}
}
