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

// EnsureIdentityFile writes ~/.config/private/git/config — the identity file
// the shared git config unconditionally includes. Missing name or email is
// asked for interactively. The file is PII, so it lives outside both the
// repository and the profile, and an existing file is never touched: real
// machines accumulate per-remote identity includes there.
func EnsureIdentityFile(ios cli.IOStreams, name, email string, gpgSign bool, home string) error {
	path := filepath.Join(home, ".config", "private", "git", "config")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", path, err)
	}

	if name == "" {
		var err error
		if name, err = tui.Input(ios, "Git identity: your name?", "Ada Lovelace", nil); err != nil || name == "" {
			tui.Warnf(ios, "No git identity configured; commits need %s (or pass --git-name/--git-email)", path)
			return nil
		}
	}
	if email == "" {
		var err error
		if email, err = tui.Input(ios, "Git identity: your email?", "ada@example.com", nil); err != nil || email == "" {
			tui.Warnf(ios, "No git identity configured; commits need %s (or pass --git-name/--git-email)", path)
			return nil
		}
	}

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
