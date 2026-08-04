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

// Scaffold creates or completes a private repository at repo with an empty
// profile named profileName. Existing files are left alone, so adopting a
// repository and re-running init are both safe.
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
	for _, dir := range []string{AgeDir(repo, profileName), HomeDir(repo, profileName)} {
		if err := cli.EnsureDir(dir, 0o755); err != nil {
			return err
		}
		// Git tracks files only; the placeholder keeps the empty profile
		// shape committable.
		keep := filepath.Join(dir, ".gitkeep")
		if _, err := os.Stat(keep); os.IsNotExist(err) {
			if err := cli.AtomicWriteFile(keep, nil, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
