// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"context"
	"fmt"
	"os"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// Rekey re-encrypts every ciphertext a profile carries to its current
// recipients and returns how many files it rewrote. It runs after an enroll
// or de-enroll: existing ciphertext never learns about a new key on its
// own, and a removed key must stop being able to open future revisions.
// Decryption needs one currently-enrolled key present (or a software
// identity), so callers gate with RequireTerminal first.
//
// Manifest entries whose plaintext is unchanged get their ciphertext hash
// refreshed — a rekey rewrites every file, and without the refresh the next
// link would re-decrypt the whole tree (and re-prompt for the PIN) for
// nothing.
func Rekey(ctx context.Context, r Runner, repo, profile, dataDir string) (int, error) {
	files, err := ListFiles(repo, profile)
	if err != nil {
		return 0, err
	}
	identities, err := Identities(repo, profile)
	if err != nil {
		return 0, err
	}
	manifestPath := ManifestPath(dataDir, profile)
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return 0, err
	}
	recipients := RecipientsPath(repo, profile)
	count := 0
	for _, f := range files {
		if !f.Encrypted {
			continue
		}
		ct, err := os.ReadFile(f.RepoPath)
		if err != nil {
			return count, fmt.Errorf("read %s: %w", f.RepoPath, err)
		}
		pt, err := Decrypt(ctx, r, identities, ct)
		if err != nil {
			return count, fmt.Errorf("%s: %w", f.Rel, err)
		}
		rekeyed, err := Encrypt(ctx, r, recipients, pt)
		if err != nil {
			return count, fmt.Errorf("%s: %w", f.Rel, err)
		}
		if err := cli.AtomicWriteFile(f.RepoPath, rekeyed, 0o644); err != nil {
			return count, err
		}
		if entry, ok := manifest[f.Rel]; ok && entry.PlainHash == HashBytes(pt) {
			entry.CipherHash = HashBytes(rekeyed)
			manifest[f.Rel] = entry
		}
		count++
	}
	if err := manifest.Save(manifestPath); err != nil {
		return count, err
	}
	return count, nil
}
