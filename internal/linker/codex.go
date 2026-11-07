// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// MigrateCodexConfig converts a legacy dotty-linked ~/.config/codex/config.toml
// into a real machine-local file Codex owns. Older layouts symlinked the live
// config through the active profile, so Codex's machine state (project trust,
// hooks.state, desktop-app MCP servers) leaked into the shared repository;
// dotty's settings now render as dotty.config.toml, which Codex reads through
// its system config layer (see LinkCodexSystemConfig). Only the exact symlink dotty wrote is migrated: its content is
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
		"dotty's settings live in dotty.config.toml, read through %s",
		site, backupDir, CodexSystemConfig)
	return nil
}

// CodexSystemConfig is Codex's system config layer. Codex reads it on every
// invocation — interactive, exec, doctor, scripts, and the desktop app —
// beneath $CODEX_HOME/config.toml, which overrides it key by key.
const CodexSystemConfig = "/etc/codex/config.toml"

// Runner runs the elevated commands the system link needs. RunInteractive
// because sudo prompts for a password on the terminal.
type Runner interface {
	RunInteractive(ctx context.Context, name string, args ...string) error
}

// LinkCodexSystemConfig points site (CodexSystemConfig outside tests) at the
// active profile's rendered dotty.config.toml, so every codex invocation picks
// up dotty's settings without a flag while $CODEX_HOME/config.toml stays the
// machine-local file Codex owns. The link resolves through active-profile, so
// profile activate retargets Codex along with everything else, and creating
// it is the only step that needs sudo. A profile without codex, or a link
// already in place, is a no-op; a file or foreign symlink at site is never
// replaced — it may be managed by someone else — only reported. Once linked,
// it warns about hardening keys in config.toml that would shadow the layer.
func LinkCodexSystemConfig(ctx context.Context, ios cli.IOStreams, r Runner, site, home, configDir string) error {
	target := filepath.Join(configDir, "active-profile", "home", ".config", "codex", "dotty.config.toml")
	if _, err := os.Stat(target); errors.Is(err, fs.ErrNotExist) {
		return nil // the active profile does not select codex
	} else if err != nil {
		return fmt.Errorf("inspect %s: %w", target, err)
	}

	_, err := os.Lstat(site)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		tui.Infof(ios, "Linking Codex's system config %s to dotty's settings needs sudo", site)
		for _, args := range [][]string{{"mkdir", "-p", filepath.Dir(site)}, {"ln", "-s", target, site}} {
			if err := r.RunInteractive(ctx, "sudo", args...); err != nil {
				return fmt.Errorf("sudo %s: %w", strings.Join(args, " "), err)
			}
		}
		tui.Successf(ios, "Codex reads dotty's settings through %s", site)
	case err != nil:
		return fmt.Errorf("inspect %s: %w", site, err)
	default:
		// Readlink fails on a regular file, which leaves link empty and so
		// lands in the foreign case below.
		if link, _ := os.Readlink(site); link != target {
			tui.Warnf(ios, "%s is not dotty's link, so Codex ignores dotty's settings; "+
				"replace it with: sudo ln -sfn %s %s", site, target, site)
			return nil
		}
	}

	machine := filepath.Join(home, ".config", "codex", "config.toml")
	keys, err := codexShadowedKeys(machine)
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		tui.Warnf(ios, "%s sets %s, overriding dotty's Codex sandbox and approvals; remove them to restore the profile's",
			machine, strings.Join(keys, ", "))
	}
	return nil
}

// codexHardeningKeys are the top-level keys dotty's layer sets to confine
// Codex. A value for any of them in config.toml wins over the system layer —
// typically left there by the pre-layer layout, whose migrated config.toml
// carries a full copy of the old render. Other keys (model, reasoning effort)
// are ones Codex writes itself when the user changes them, so their winning is
// intended.
var codexHardeningKeys = []string{"approval_policy", "sandbox_mode", "sandbox_workspace_write"}

// codexShadowedKeys returns the hardening keys path sets, either as top-level
// assignments or as tables. A missing file shadows nothing. It is a line scan,
// not a TOML parse: config.toml is Codex's own and only needs inspecting.
func codexShadowedKeys(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var found []string
	topLevel := true
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		var key string
		if header, ok := strings.CutPrefix(line, "["); ok {
			topLevel = false
			key, _, _ = strings.Cut(strings.TrimLeft(header, "["), "]")
			key, _, _ = strings.Cut(key, ".")
		} else if k, _, ok := strings.Cut(line, "="); ok && topLevel && !strings.HasPrefix(line, "#") {
			key, _, _ = strings.Cut(strings.TrimSpace(k), ".")
		}
		key = strings.TrimSpace(key)
		if slices.Contains(codexHardeningKeys, key) && !slices.Contains(found, key) {
			found = append(found, key)
		}
	}
	return found, nil
}
