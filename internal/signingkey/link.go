// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package signingkey

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DefaultLinkPath is where `signing-key link` points its symlink when no path
// is given: an ssh identity name under ~/.ssh, so ssh's IdentityFile can name
// this fixed path while the link follows whichever YubiKey is connected.
func DefaultLinkPath(home string) string {
	return filepath.Join(home, ".ssh", "id_sk_current")
}

// Link atomically points linkPath at ref's stub and linkPath+".pub" at its
// public key, so a fixed ssh IdentityFile resolves to the connected key. ssh
// derives the public-key path by appending ".pub", so both links are written.
// Existing links are replaced without a window where they are missing.
func Link(linkPath string, ref KeyRef) error {
	if err := replaceSymlink(ref.PrivPath, linkPath); err != nil {
		return err
	}
	return replaceSymlink(ref.PubPath, linkPath+".pub")
}

// replaceSymlink points link at target, creating parent dirs and swapping any
// existing link in via a temp link and rename so a concurrent reader never
// observes link missing.
func replaceSymlink(target, link string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		return fmt.Errorf("create link dir: %w", err)
	}
	tmp := link + ".tmp"
	if err := os.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("clear stale temp link: %w", err)
	}
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("create link: %w", err)
	}
	if err := os.Rename(tmp, link); err != nil {
		return fmt.Errorf("replace link: %w", err)
	}
	return nil
}
