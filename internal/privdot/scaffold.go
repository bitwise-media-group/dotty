// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/scaffold"
	"github.com/bitwise-media-group/dotty/internal/version"
)

// marker is the PrivateMarker document: the release that scaffolded the
// repository, for future upgrades — the private analogue of .dotty-version.
type marker struct {
	Version string `json:"version"`
}

// gitattributes keeps ciphertext out of text-mode line rewriting and
// textual merges: an eol filter or a merge marker inside an age file
// corrupts it.
const gitattributes = `* text=auto eol=lf
*.age -text -diff -merge
`

// gitignore blocks the classic plaintext accidents. The real guard is
// behavioral — dotty never materializes plaintext inside the working tree —
// but a stray decrypt or key dump should not reach the index either.
const gitignore = `.DS_Store
*.dec
*.plain
known_hosts
authorized_keys
id_*
!id_*.pub
`

// preCommitHook rejects commits that verify finds unsafe; init points
// core.hooksPath at .githooks so the hook travels with the repository.
const preCommitHook = `#!/usr/bin/env sh
set -eu
exec dotty private verify --staged
`

// skeletonDirs are the $HOME-relative directories every profile's home tree
// starts with — the drop-in sites the public templates already include
// (`Include ~/.ssh/config.d/*.conf` in the ssh config, `path =
// ~/.config/private/git/config` in the git config), so the places dotty
// private link deploys into are visible before the first encrypt.
var skeletonDirs = []string{
	filepath.Join(".ssh", "config.d"),
	filepath.Join(".config", "private", "git"),
}

// Scaffold creates or completes a private repository at repo with a profile
// named profileName carrying the skeletonDirs drop-in shape. Existing files
// are left alone, so adopting a repository and re-running init are both safe.
func Scaffold(repo, profileName string) error {
	if err := cli.EnsureDir(repo, 0o755); err != nil {
		return err
	}
	markerPath := filepath.Join(repo, scaffold.PrivateMarker)
	if _, err := os.Stat(markerPath); os.IsNotExist(err) {
		data, err := json.Marshal(marker{Version: version.Version})
		if err != nil {
			return fmt.Errorf("encode %s: %w", scaffold.PrivateMarker, err)
		}
		if err := cli.AtomicWriteFile(markerPath, append(data, '\n'), 0o644); err != nil {
			return err
		}
	}
	seeds := map[string]struct {
		content string
		perm    os.FileMode
	}{
		".gitattributes":       {gitattributes, 0o644},
		".gitignore":           {gitignore, 0o644},
		".githooks/pre-commit": {preCommitHook, 0o755},
	}
	for rel, seed := range seeds {
		path := filepath.Join(repo, rel)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := cli.EnsureDir(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := cli.AtomicWriteFile(path, []byte(seed.content), seed.perm); err != nil {
			return err
		}
	}
	home := HomeDir(repo, profileName)
	dirs := []string{AgeDir(repo, profileName)}
	for _, rel := range skeletonDirs {
		dirs = append(dirs, filepath.Join(home, rel))
	}
	for _, dir := range dirs {
		if err := ensureKept(dir); err != nil {
			return err
		}
	}
	return nil
}

// ensureKept creates dir and, while it holds nothing else, a .gitkeep
// placeholder — git tracks files only, and the placeholder keeps the empty
// shape committable without cluttering directories that already have content.
func ensureKept(dir string) error {
	if err := cli.EnsureDir(dir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	if len(entries) > 0 {
		return nil
	}
	return cli.AtomicWriteFile(filepath.Join(dir, gitkeepName), nil, 0o644)
}
