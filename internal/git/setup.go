// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// InitRepo turns dir into a git repository with everything staged, unless it
// already is one. No commit: the first commit should carry the user's
// signature, and signing may not be configured yet.
func InitRepo(ctx context.Context, r Runner, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	if _, err := r.Output(ctx, "git", "-C", dir, "init", "-b", "main"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	if _, err := r.Output(ctx, "git", "-C", dir, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

// IdentityPath returns the private git identity file's location —
// ~/.config/private/git/config, the file the shared git config
// unconditionally includes. It is PII, so it lives outside both the
// repository and the profile.
func IdentityPath(home string) string {
	return filepath.Join(home, ".config", "private", "git", "config")
}

// NeedsIdentity reports whether the private identity file is missing — the
// only case init writes one. An existing file is never touched: real
// machines accumulate per-remote identity includes there.
func NeedsIdentity(home string) (bool, error) {
	if _, err := os.Stat(IdentityPath(home)); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("inspect %s: %w", IdentityPath(home), err)
	}
	return true, nil
}

// WriteIdentityFile writes the private identity file with the given name and
// email; the interview supplies them, so nothing is asked here.
func WriteIdentityFile(ios cli.IOStreams, name, email string, gpgSign bool, home string) error {
	path := IdentityPath(home)
	if err := cli.EnsureDir(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	content := fmt.Sprintf("[user]\n\tname = %s\n\temail = %s\n\n[commit]\n\tgpgSign = %t\n\n[tag]\n\tgpgSign = %t\n",
		name, email, gpgSign, gpgSign)
	if err := cli.AtomicWriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	tui.Successf(ios, "Wrote git identity to %s (gpgSign = %t)", path, gpgSign)
	return nil
}
