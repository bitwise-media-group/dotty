// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/bitwise-media-group/dotty/internal/mise"
	"github.com/bitwise-media-group/dotty/internal/profile"
)

// fakeMise is a mise stand-in: it appends each invocation, prefixed with
// the MISE_CONFIG_DIR it was given, to $DOTTY_TEST_MISE_LOG; answers
// `bootstrap packages prune --dry-run` with $DOTTY_TEST_PRUNE; and writes
// $DOTTY_TEST_IMPORT to the --path of `bootstrap packages import`.
const fakeMise = `#!/bin/sh
set -eu
if [ -n "${DOTTY_TEST_MISE_LOG-}" ]; then
	printf '%s|%s\n' "${MISE_CONFIG_DIR-}" "$*" >>"$DOTTY_TEST_MISE_LOG"
fi
case "$*" in
"bootstrap packages prune --dry-run")
	printf '%s\n' "${DOTTY_TEST_PRUNE-}"
	;;
"bootstrap packages import "*)
	path=""
	while [ $# -gt 0 ]; do
		if [ "$1" = "--path" ]; then path="$2"; fi
		shift
	done
	printf '%s\n' "${DOTTY_TEST_IMPORT-}" >"$path"
	;;
esac
`

// installFakeMise writes the fake mise where EnsureInstalled looks first,
// ~/.local/bin/mise, so no test ever downloads the real installer.
func installFakeMise(t *testing.T, home string) {
	t.Helper()
	bin := mise.LocalBin(home)
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte(fakeMise), 0o755); err != nil {
		t.Fatal(err)
	}
}

// packagesEnv points dotty at a scratch active profile whose mise config.toml
// holds config (the template default when empty) and whose conf.d holds
// fragments, installs the fake mise, and returns the profile's mise
// directory plus the log recording every mise invocation.
func packagesEnv(t *testing.T, config string, fragments map[string]string) (dir, logPath string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	xdg := filepath.Join(home, ".config")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	configDir := filepath.Join(xdg, "dotty")
	if _, err := profile.Create(configDir, "personal", "test profile"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("personal", filepath.Join(configDir, "active-profile")); err != nil {
		t.Fatal(err)
	}
	dir = mise.Dir(profile.Dir(configDir, "personal"))
	if err := os.MkdirAll(mise.ConfDDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if config != "" {
		if err := os.WriteFile(mise.ConfigPath(dir), []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range fragments {
		if err := os.WriteFile(filepath.Join(mise.ConfDDir(dir), name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	installFakeMise(t, home)
	logPath = filepath.Join(home, "mise.log")
	t.Setenv("DOTTY_TEST_MISE_LOG", logPath)
	t.Setenv("DOTTY_TEST_PRUNE", "")
	t.Setenv("DOTTY_TEST_IMPORT", "")

	// Command flags are package-level, so a run leaves its values — and
	// cobra's changed bits, which the mutual-exclusion check reads — behind
	// for the next one.
	t.Cleanup(func() {
		for _, cmd := range []*cobra.Command{packagesRemoveCmd, packagesSyncCmd, packagesImportCmd} {
			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				_ = f.Value.Set(f.DefValue)
				f.Changed = false
			})
		}
		rootFlags.Profile = ""
	})
	return dir, logPath
}

// miseCalls returns the logged fake-mise invocations as "<args>" lines,
// checking that each carried the profile's mise directory.
func miseCalls(t *testing.T, logPath, dir string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for line := range strings.Lines(strings.TrimSpace(string(data))) {
		env, args, _ := strings.Cut(strings.TrimRight(line, "\n"), "|")
		if env != dir {
			t.Errorf("call %q ran with MISE_CONFIG_DIR=%q, want %q", args, env, dir)
		}
		calls = append(calls, args)
	}
	return calls
}

const testPackagesConfig = `[settings]
lockfile = true

[tools]
"aqua:sharkdp/fd" = "latest"

[bootstrap.packages]
"brew:git" = "latest"
"brew-cask:ghostty" = { version = "latest", os = "macos" }
`

var testPackagesFragments = map[string]string{
	"core.toml": "[tools]\n\"aqua:jqlang/jq\" = \"latest\"\n\n[bootstrap.packages]\n\"brew:curl\" = \"latest\"\n",
}

// TestPackagesAdd runs `dotty packages add` end-to-end against the fake
// mise, pinning the id validation and the tool/package dispatch.
func TestPackagesAdd(t *testing.T) {
	t.Run("bare names are refused", func(t *testing.T) {
		_, logPath := packagesEnv(t, testPackagesConfig, nil)
		err := execDotty(t, "packages", "add", "bat")
		if err == nil || !strings.Contains(err.Error(), "mise registry bat") {
			t.Fatalf("add bat = %v, want a full-id error with the registry hint", err)
		}
		if calls := miseCalls(t, logPath, ""); len(calls) != 0 {
			t.Errorf("mise calls = %v, want none", calls)
		}
	})

	t.Run("dispatches tools and packages", func(t *testing.T) {
		dir, logPath := packagesEnv(t, testPackagesConfig, testPackagesFragments)
		if err := execDotty(t, "packages", "add", "aqua:sharkdp/bat", "brew:wget", "aqua:jqlang/jq"); err != nil {
			t.Fatalf("add: %v", err)
		}
		want := []string{
			"use --global --yes aqua:sharkdp/bat",
			"lock --global",
			"bootstrap packages use --global --yes brew:wget",
		}
		if got := miseCalls(t, logPath, dir); !slices.Equal(got, want) {
			t.Errorf("mise calls = %v, want %v", got, want)
		}
	})
}

// TestPackagesRemove runs `dotty packages remove` end-to-end against the
// fake mise, pinning the config edit, the fragment refusal, and the
// no-id picker guards.
func TestPackagesRemove(t *testing.T) {
	t.Run("removes a package by editing the config", func(t *testing.T) {
		dir, logPath := packagesEnv(t, testPackagesConfig, nil)
		if err := execDotty(t, "packages", "remove", "brew-cask:ghostty"); err != nil {
			t.Fatalf("remove: %v", err)
		}
		got, err := os.ReadFile(mise.ConfigPath(dir))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(got), "ghostty") || !strings.Contains(string(got), `"brew:git"`) {
			t.Errorf("config after remove = %q, want ghostty gone and git kept", got)
		}
		if calls := miseCalls(t, logPath, dir); len(calls) != 0 {
			t.Errorf("mise calls = %v, want none for a package removal", calls)
		}
	})

	t.Run("removes a tool through unuse", func(t *testing.T) {
		dir, logPath := packagesEnv(t, testPackagesConfig, nil)
		if err := execDotty(t, "packages", "rm", "aqua:sharkdp/fd"); err != nil {
			t.Fatalf("remove: %v", err)
		}
		if got := miseCalls(t, logPath, dir); !slices.Equal(got, []string{"unuse --global aqua:sharkdp/fd"}) {
			t.Errorf("mise calls = %v", got)
		}
	})

	t.Run("fragment entries are refused", func(t *testing.T) {
		dir, logPath := packagesEnv(t, testPackagesConfig, testPackagesFragments)
		if err := execDotty(t, "packages", "remove", "aqua:jqlang/jq", "brew:nope"); err != nil {
			t.Fatalf("remove: %v", err)
		}
		if calls := miseCalls(t, logPath, dir); len(calls) != 0 {
			t.Errorf("mise calls = %v, want none for managed or unknown ids", calls)
		}
		if _, err := os.Stat(filepath.Join(mise.ConfDDir(dir), "core.toml")); err != nil {
			t.Errorf("fragment touched: %v", err)
		}
	})

	t.Run("no ids and no terminal is an error", func(t *testing.T) {
		packagesEnv(t, testPackagesConfig, nil)
		err := execDotty(t, "packages", "remove")
		if err == nil || !strings.Contains(err.Error(), "no terminal") {
			t.Fatalf("remove without ids = %v, want a no-terminal picker error", err)
		}
	})

	t.Run("no ids and no own entries is a no-op", func(t *testing.T) {
		dir, logPath := packagesEnv(t, "", testPackagesFragments)
		if err := execDotty(t, "packages", "remove"); err != nil {
			t.Fatalf("remove with nothing to pick: %v", err)
		}
		if calls := miseCalls(t, logPath, dir); len(calls) != 0 {
			t.Errorf("mise calls = %v, want none", calls)
		}
	})
}

// TestPackagesSync pins the sync argv sequence and the non-interactive
// removal guard.
func TestPackagesSync(t *testing.T) {
	t.Run("force", func(t *testing.T) {
		dir, logPath := packagesEnv(t, testPackagesConfig, nil)
		if err := execDotty(t, "packages", "sync", "--force"); err != nil {
			t.Fatalf("sync --force: %v", err)
		}
		want := []string{
			"lock --global", "install --yes", "prune --yes",
			"bootstrap packages apply --yes", "bootstrap packages prune --yes",
		}
		if got := miseCalls(t, logPath, dir); !slices.Equal(got, want) {
			t.Errorf("mise calls = %v, want %v", got, want)
		}
	})

	t.Run("nothing to remove", func(t *testing.T) {
		dir, logPath := packagesEnv(t, testPackagesConfig, nil)
		if err := execDotty(t, "packages", "sync"); err != nil {
			t.Fatalf("sync: %v", err)
		}
		want := []string{
			"bootstrap packages prune --dry-run", "lock --global", "install --yes", "prune --yes",
			"bootstrap packages apply --yes",
		}
		if got := miseCalls(t, logPath, dir); !slices.Equal(got, want) {
			t.Errorf("mise calls = %v, want %v", got, want)
		}
	})

	t.Run("removals need a terminal or --force", func(t *testing.T) {
		dir, logPath := packagesEnv(t, testPackagesConfig, nil)
		t.Setenv("DOTTY_TEST_PRUNE", "remove brew:mise@2026.9.1")
		err := execDotty(t, "packages", "sync")
		if err == nil || !strings.Contains(err.Error(), "--force") {
			t.Fatalf("sync with removals = %v, want the --force hint", err)
		}
		if got := miseCalls(t, logPath, dir); !slices.Equal(got, []string{"bootstrap packages prune --dry-run"}) {
			t.Errorf("mise calls = %v, want the dry run only", got)
		}
	})
}

// TestPackagesImport pins both import sources: the installed formulae via
// mise (skipping what the profile declares) and a Brewfile via the
// converter.
func TestPackagesImport(t *testing.T) {
	t.Run("installed formulae", func(t *testing.T) {
		dir, logPath := packagesEnv(t, testPackagesConfig, testPackagesFragments)
		t.Setenv("DOTTY_TEST_IMPORT", "[bootstrap.packages]\n\"brew:git\" = \"latest\"\n"+
			"\"brew:curl\" = \"latest\"\n\"brew:wget\" = \"latest\"\n")
		if err := execDotty(t, "packages", "import", "--all"); err != nil {
			t.Fatalf("import: %v", err)
		}
		got, err := os.ReadFile(mise.ConfigPath(dir))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "# installed packages\n\"brew:wget\" = \"latest\"\n") {
			t.Errorf("imported entry missing:\n%s", got)
		}
		if strings.Count(string(got), `"brew:git"`) != 1 || strings.Contains(string(got), `"brew:curl"`) {
			t.Errorf("declared entries duplicated:\n%s", got)
		}
		calls := miseCalls(t, logPath, dir)
		if len(calls) != 1 || !strings.HasPrefix(calls[0], "bootstrap packages import --manager brew --path ") ||
			!strings.HasSuffix(calls[0], " --all") {
			t.Errorf("mise calls = %v", calls)
		}
	})

	t.Run("brewfile", func(t *testing.T) {
		dir, logPath := packagesEnv(t, testPackagesConfig, testPackagesFragments)
		brewfile := filepath.Join(t.TempDir(), "Brewfile")
		if err := os.WriteFile(brewfile, []byte("brew \"jq\"\nbrew \"colima\"\ncask \"raycast\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := execDotty(t, "packages", "import", "--brewfile="+brewfile); err != nil {
			t.Fatalf("import --brewfile: %v", err)
		}
		got, err := os.ReadFile(mise.ConfigPath(dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"# imported from Brewfile\n\"brew:colima\" = \"latest\"\n",
			`"brew-cask:raycast" = { version = "latest", os = "macos" }`} {
			if !strings.Contains(string(got), want) {
				t.Errorf("converted entry %q missing:\n%s", want, got)
			}
		}
		if strings.Contains(string(got), "jqlang") {
			t.Errorf("fragment-covered formula converted anyway:\n%s", got)
		}
		if calls := miseCalls(t, logPath, dir); len(calls) != 0 {
			t.Errorf("mise calls = %v, want none for a Brewfile conversion", calls)
		}
	})
}

// TestResolvePackagesDir pins where the packages verbs look: the --profile
// flag's profile when given (and it must exist), otherwise the active
// profile (and one must be active) — and that a profile with no mise config
// yet gets the default one.
func TestResolvePackagesDir(t *testing.T) {
	cases := []struct {
		name        string
		flagProfile string
		active      bool
		wantProfile string
		wantErr     error
	}{
		{name: "explicit profile", flagProfile: "work", active: true, wantProfile: "work"},
		{name: "missing explicit profile", flagProfile: "nope", active: true, wantErr: profile.ErrNotFound},
		{name: "active profile", active: true, wantProfile: "personal"},
		{name: "no active profile", wantErr: profile.ErrNoActiveProfile},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			xdg := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", xdg)
			configDir := filepath.Join(xdg, "dotty")
			for _, name := range []string{"personal", "work"} {
				if _, err := profile.Create(configDir, name, "test profile"); err != nil {
					t.Fatal(err)
				}
			}
			if c.active {
				if err := os.Symlink("personal", filepath.Join(configDir, "active-profile")); err != nil {
					t.Fatal(err)
				}
			}
			rootFlags.Profile = c.flagProfile
			t.Cleanup(func() { rootFlags.Profile = "" })

			got, err := resolvePackagesDir()
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("resolvePackagesDir() error = %v, want %v", err, c.wantErr)
			}
			if c.wantErr != nil {
				return
			}
			want := mise.Dir(profile.Dir(configDir, c.wantProfile))
			if got != want {
				t.Errorf("resolvePackagesDir() = %q, want %q", got, want)
			}
			if data, err := os.ReadFile(mise.ConfigPath(got)); err != nil || string(data) != mise.DefaultConfig {
				t.Errorf("default config not created: %v", err)
			}
		})
	}
}
