// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnvOldVerbsGone pins the breaking change: the verbs fnox replaced are
// no longer commands, so a stale script fails loudly rather than doing
// something else.
func TestEnvOldVerbsGone(t *testing.T) {
	for _, verb := range []string{"add", "get", "list", "remove", "run", "use"} {
		t.Run(verb, func(t *testing.T) {
			err := execDotty(t, "env", verb)
			if err == nil || !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("execute env %s = %v, want unknown command error", verb, err)
			}
		})
	}
}

// TestEnvMigrateValidatesPathsFirst pins that a missing template fails
// before fnox is consulted: with no fnox on PATH the error is still about
// the path, not the tool.
func TestEnvMigrateValidatesPathsFirst(t *testing.T) {
	t.Cleanup(func() { envMigrateFlags = EnvMigrateFlags{} })
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())
	err := execDotty(t, "env", "migrate", "nonexistent.env")
	if err == nil || !strings.Contains(err.Error(), "template nonexistent.env") {
		t.Fatalf("execute = %v, want template path error", err)
	}
}

// TestEnvMigrateNeedsFnox pins the install hint when fnox is absent, on a
// path that touches neither the keychain nor a template.
func TestEnvMigrateNeedsFnox(t *testing.T) {
	t.Cleanup(func() { envMigrateFlags = EnvMigrateFlags{} })
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())
	err := execDotty(t, "env", "migrate", "--skip-keychain")
	if err == nil || !strings.Contains(err.Error(), "dotty packages sync installs it") {
		t.Fatalf("execute = %v, want fnox install hint", err)
	}
}

// TestEnvMigrateFiles pins the template fallback: the working directory's
// .env.dotty when no PATH is given, nothing when there is none, and every
// explicit PATH checked for existence.
func TestEnvMigrateFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, defaultEnvFile), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Run("falls back to .env.dotty", func(t *testing.T) {
		t.Chdir(dir)
		got, err := envMigrateFiles(nil)
		if err != nil || len(got) != 1 || got[0] != defaultEnvFile {
			t.Fatalf("envMigrateFiles(nil) = %v, %v; want [%s]", got, err, defaultEnvFile)
		}
	})
	t.Run("none without .env.dotty", func(t *testing.T) {
		t.Chdir(t.TempDir())
		got, err := envMigrateFiles(nil)
		if err != nil || len(got) != 0 {
			t.Fatalf("envMigrateFiles(nil) = %v, %v; want none", got, err)
		}
	})
	t.Run("explicit paths must exist", func(t *testing.T) {
		t.Chdir(dir)
		if _, err := envMigrateFiles([]string{defaultEnvFile, "missing.env"}); err == nil {
			t.Fatal("envMigrateFiles with a missing path = nil error")
		}
	})
}
