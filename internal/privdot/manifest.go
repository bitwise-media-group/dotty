// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// Entry records one decrypted file's provenance so a re-link can tell what
// changed without touching hardware: the ciphertext hash gates re-decryption
// (every decrypt can cost a PIN or touch), and the plaintext hash detects
// local edits that were never encrypted back.
type Entry struct {
	CipherHash  string    `json:"cipherHash,omitempty"` // empty for repo files deployed as-is
	PlainHash   string    `json:"plainHash"`
	DecryptedAt time.Time `json:"decryptedAt"`
}

// Manifest maps a profile's $HOME-relative paths to their decrypt records.
type Manifest map[string]Entry

// LoadManifest reads a profile's manifest; a missing file is an empty
// manifest — the profile has simply never been decrypted here.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Manifest{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

// Save writes the manifest 0600 — it holds plaintext hashes, which stay
// inside the 0700 data directory with the plaintext itself.
func (m Manifest) Save(path string) error {
	if err := cli.EnsureDir(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	return cli.AtomicWriteFile(path, append(data, '\n'), 0o600)
}

// HashBytes returns the hex sha256 of data.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HashFile returns the hex sha256 of the file at path.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
