// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// MigrateCodexConfig converts a legacy dotty-linked ~/.config/codex/config.toml
// into a real machine-local file Codex owns. Older layouts symlinked the live
// config through the active profile, so Codex's machine state (project trust,
// hooks.state, desktop-app MCP servers) leaked into the shared repository;
// dotty's settings now render as the codex profile layer dotty.config.toml
// instead. Only the exact symlink dotty wrote is migrated: its content is
// mirrored into a backup set and rewritten in place as a regular file, so the
// trust decisions and desktop integration survive. A missing site, a real
// file, or a foreign symlink is left untouched, which makes the migration
// idempotent. Must run before the render prunes the old per-profile file —
// that file is the very content the link dereferences to.
func MigrateCodexConfig(ios cli.IOStreams, home, configDir string) error {
	site := filepath.Join(home, ".config", "codex", "config.toml")
	info, err := os.Lstat(site)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect %s: %w", site, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return nil // already migrated, or a user-owned real file
	}

	target, err := os.Readlink(site)
	if err != nil {
		return fmt.Errorf("read link %s: %w", site, err)
	}
	owned := filepath.Join(configDir, "active-profile", "home", ".config", "codex", "config.toml")
	if target != owned {
		return nil // not dotty's link; leave the user's own symlink alone
	}

	content, err := os.ReadFile(site)
	if errors.Is(err, fs.ErrNotExist) {
		// Dangling — the profile behind it was already switched or pruned, so
		// there is no machine state to preserve.
		if err := os.Remove(site); err != nil {
			return fmt.Errorf("remove dangling %s: %w", site, err)
		}
		tui.Infof(ios, "Removed dangling link %s; Codex now owns that path", site)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", site, err)
	}

	dataDir, err := cli.DataDir()
	if err != nil {
		return err
	}
	backupDir := filepath.Join(dataDir, "backups", time.Now().Format("2006-01-02T15-04-05"))
	backup := filepath.Join(backupDir, strings.TrimPrefix(site, string(filepath.Separator)))
	if err := cli.EnsureDir(filepath.Dir(backup), 0o700); err != nil {
		return err
	}
	// Copy rather than move: moving would relocate the symlink, not the
	// profile content it dereferences to.
	if err := cli.AtomicWriteFile(backup, content, 0o644); err != nil {
		return fmt.Errorf("back up %s: %w", site, err)
	}

	if err := os.Remove(site); err != nil {
		return fmt.Errorf("unlink %s: %w", site, err)
	}
	if err := cli.AtomicWriteFile(site, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", site, err)
	}
	tui.Infof(ios, "Migrated %s to a machine-local file Codex owns (backup under %s); "+
		"dotty's settings live in dotty.config.toml and take precedence under --profile dotty",
		site, backupDir)
	return nil
}
