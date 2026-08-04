// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// codexState is the representative machine-local content Codex writes into
// its config — the state the migration must carry over.
const codexState = "[projects.\"/x\"]\ntrust_level = \"trusted\"\n"

// migrateEnv builds a fake home with the config/data dirs MigrateCodexConfig
// resolves, returning the home, dotty config dir, and live codex site path.
func migrateEnv(t *testing.T) (home, configDir, site string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	configDir = filepath.Join(home, ".config", "dotty")
	site = filepath.Join(home, ".config", "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(site), 0o755); err != nil {
		t.Fatal(err)
	}
	return home, configDir, site
}

// migrateIOS returns buffered IOStreams so the migration's messages never
// reach the test output.
func migrateIOS() cli.IOStreams {
	return cli.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
}

// backupSets returns the backup set directories under the fake data dir.
func backupSets(t *testing.T, home string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(home, ".local", "share", "dotty", "backups"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestMigrateCodexConfigMissingSite(t *testing.T) {
	home, configDir, site := migrateEnv(t)
	if err := MigrateCodexConfig(migrateIOS(), home, configDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(site); !os.IsNotExist(err) {
		t.Fatalf("site created: %v", err)
	}
	if sets := backupSets(t, home); len(sets) != 0 {
		t.Fatalf("backup sets created: %v", sets)
	}
}

func TestMigrateCodexConfigLeavesRealFile(t *testing.T) {
	home, configDir, site := migrateEnv(t)
	if err := os.WriteFile(site, []byte(codexState), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCodexConfig(migrateIOS(), home, configDir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(site)
	if err != nil || string(got) != codexState {
		t.Fatalf("content changed: %q, %v", got, err)
	}
	if sets := backupSets(t, home); len(sets) != 0 {
		t.Fatalf("backup sets created: %v", sets)
	}
}

func TestMigrateCodexConfigLeavesForeignSymlink(t *testing.T) {
	home, configDir, site := migrateEnv(t)
	other := filepath.Join(home, "elsewhere.toml")
	if err := os.WriteFile(other, []byte(codexState), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, site); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCodexConfig(migrateIOS(), home, configDir); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(site)
	if err != nil || target != other {
		t.Fatalf("link changed: %q, %v", target, err)
	}
}

func TestMigrateCodexConfigConvertsOwnedSymlink(t *testing.T) {
	home, configDir, site := migrateEnv(t)
	dest := filepath.Join(configDir, "active-profile", "home", ".config", "codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(codexState), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dest, site); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCodexConfig(migrateIOS(), home, configDir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(site)
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("site is not a regular file: %v, %v", info, err)
	}
	got, err := os.ReadFile(site)
	if err != nil || string(got) != codexState {
		t.Fatalf("content lost: %q, %v", got, err)
	}
	sets := backupSets(t, home)
	if len(sets) != 1 {
		t.Fatalf("want one backup set, got %v", sets)
	}
	backup := filepath.Join(home, ".local", "share", "dotty", "backups", sets[0],
		strings.TrimPrefix(site, string(filepath.Separator)))
	if got, err := os.ReadFile(backup); err != nil || string(got) != codexState {
		t.Fatalf("backup mirror missing content: %q, %v", got, err)
	}

	// A second run sees a real file and changes nothing.
	if err := MigrateCodexConfig(migrateIOS(), home, configDir); err != nil {
		t.Fatal(err)
	}
	if sets := backupSets(t, home); len(sets) != 1 {
		t.Fatalf("second run created backup sets: %v", sets)
	}
}

func TestMigrateCodexConfigRemovesDanglingOwnedSymlink(t *testing.T) {
	home, configDir, site := migrateEnv(t)
	dest := filepath.Join(configDir, "active-profile", "home", ".config", "codex", "config.toml")
	if err := os.Symlink(dest, site); err != nil {
		t.Fatal(err)
	}
	if err := MigrateCodexConfig(migrateIOS(), home, configDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(site); !os.IsNotExist(err) {
		t.Fatalf("dangling link not removed: %v", err)
	}
	if sets := backupSets(t, home); len(sets) != 0 {
		t.Fatalf("backup sets created: %v", sets)
	}
}
