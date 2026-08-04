// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// WritePlain lands one entry's plaintext in the data-dir tree, 0600 under
// 0700 directories — the only place decrypted content ever touches disk.
// The private root's permission is enforced even when it already exists;
// directories below it are always born 0700.
func WritePlain(dataDir, profile, rel string, plaintext []byte) error {
	if err := cli.EnsureDir(DataRoot(dataDir), 0o700); err != nil {
		return err
	}
	path := PlainPath(dataDir, profile, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	return cli.AtomicWriteFile(path, plaintext, 0o600)
}

// Store encrypts plaintext to the profile's recipients, writes the
// ciphertext into the repository, lands the plaintext copy in the data-dir
// tree, and records both hashes in the manifest — the whole adopt/update
// path for one entry. Plaintext reaches the repository side only through
// the encrypt pipe.
func Store(ctx context.Context, r Runner, repo, profile, dataDir, rel string, plaintext []byte) error {
	ct, err := Encrypt(ctx, r, RecipientsPath(repo, profile), plaintext)
	if err != nil {
		return err
	}
	repoPath := filepath.Join(HomeDir(repo, profile), rel+CipherExt)
	if err := cli.EnsureDir(filepath.Dir(repoPath), 0o755); err != nil {
		return err
	}
	if err := cli.AtomicWriteFile(repoPath, ct, 0o644); err != nil {
		return err
	}
	if err := WritePlain(dataDir, profile, rel, plaintext); err != nil {
		return err
	}
	manifestPath := ManifestPath(dataDir, profile)
	m, err := LoadManifest(manifestPath)
	if err != nil {
		return err
	}
	m[rel] = Entry{
		CipherHash:  HashBytes(ct),
		PlainHash:   HashBytes(plaintext),
		DecryptedAt: time.Now().UTC(),
	}
	return m.Save(manifestPath)
}
