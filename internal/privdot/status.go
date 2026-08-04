// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"errors"
	"io/fs"
)

// FileState classifies one entry's freshness across the repository, the
// manifest, and the decrypted copy.
type FileState string

const (
	// StateOK: repository, manifest, and decrypted copy agree.
	StateOK FileState = "ok"
	// StateStale: the repository changed since the last decrypt — run
	// `dotty private link`.
	StateStale FileState = "stale"
	// StateDrifted: the decrypted copy was edited locally and never
	// encrypted back — run `dotty private encrypt`.
	StateDrifted FileState = "drifted"
	// StateConflict: both sides changed; reconcile by hand before either
	// direction overwrites the other.
	StateConflict FileState = "conflict"
	// StateMissing: never decrypted on this machine — run
	// `dotty private link`.
	StateMissing FileState = "missing"
)

// FileStatus pairs an entry with its state.
type FileStatus struct {
	File
	State FileState
}

// Status classifies every entry a private profile carries. It reads hashes
// only — no decryption, no hardware.
func Status(dataDir, repo, profile string) ([]FileStatus, error) {
	files, err := ListFiles(repo, profile)
	if err != nil {
		return nil, err
	}
	manifest, err := LoadManifest(ManifestPath(dataDir, profile))
	if err != nil {
		return nil, err
	}
	statuses := make([]FileStatus, 0, len(files))
	for _, f := range files {
		state, err := classify(dataDir, profile, f, manifest)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, FileStatus{File: f, State: state})
	}
	return statuses, nil
}

// classify derives one entry's state from its three copies.
func classify(dataDir, profile string, f File, manifest Manifest) (FileState, error) {
	entry, known := manifest[f.Rel]
	plainPath := PlainPath(dataDir, profile, f.Rel)
	plainHash, err := HashFile(plainPath)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !known) {
		return StateMissing, nil
	}
	if err != nil {
		return "", err
	}
	repoHash, err := HashFile(f.RepoPath)
	if err != nil {
		return "", err
	}
	// Plaintext repo entries store their repo hash in CipherHash too — the
	// "did the repository side change" question is the same either way.
	stale := repoHash != entry.CipherHash
	drifted := plainHash != entry.PlainHash
	switch {
	case stale && drifted:
		return StateConflict, nil
	case stale:
		return StateStale, nil
	case drifted:
		return StateDrifted, nil
	}
	return StateOK, nil
}
