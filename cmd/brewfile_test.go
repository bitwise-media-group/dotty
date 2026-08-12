// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/brewfile"
	"github.com/bitwise-media-group/dotty/internal/profile"
)

// fakeBrew is a brew stand-in installed on PATH: it appends each invocation to
// $DOTTY_TEST_BREW_LOG, answers `bundle list` by reading `brew "..."` lines
// straight from the --file Brewfile, edits the file in place for
// `bundle remove`, and exits $DOTTY_TEST_UNTRUST_EXIT for `untrust`.
const fakeBrew = `#!/bin/sh
set -eu
printf '%s\n' "$*" >>"$DOTTY_TEST_BREW_LOG"
case "${1-}" in
bundle)
	verb="$2"; shift 2
	file=""; names=""
	for a in "$@"; do
		case "$a" in
		--file=*) file="${a#--file=}" ;;
		--*) ;;
		*) names="$names $a" ;;
		esac
	done
	case "$verb" in
	list)
		sed -n 's/^brew "\([^"]*\)".*/\1/p' "$file"
		;;
	remove)
		for n in $names; do
			grep -v "^brew \"$n\"" "$file" >"$file.new" || :
			mv "$file.new" "$file"
		done
		;;
	esac
	;;
untrust)
	exit "${DOTTY_TEST_UNTRUST_EXIT:-0}"
	;;
esac
`

// brewfileEnv points dotty at a scratch active profile whose Brewfile holds
// lines (no file at all when empty), puts the fake brew first on PATH, and
// returns the Brewfile path plus the log recording every brew invocation.
func brewfileEnv(t *testing.T, lines ...string) (brewfilePath, logPath string) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	configDir := filepath.Join(xdg, "dotty")
	if _, err := profile.Create(configDir, "personal", "test profile"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("personal", filepath.Join(configDir, "active-profile")); err != nil {
		t.Fatal(err)
	}
	brewfilePath = profile.BrewfilePath(profile.Dir(configDir, "personal"))
	if len(lines) > 0 {
		if err := os.WriteFile(brewfilePath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "brew"), []byte(fakeBrew), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	logPath = filepath.Join(binDir, "brew.log")
	t.Setenv("DOTTY_TEST_BREW_LOG", logPath)

	// Command flags are package-level, so a run leaves its values behind for
	// the next one.
	t.Cleanup(func() {
		brewfileRemoveFlags = BrewfileRemoveFlags{}
		rootFlags.Profile = ""
	})
	return brewfilePath, logPath
}

// brewCalls returns the logged fake-brew invocations, one argv line each.
func brewCalls(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// TestBrewfileRemove runs `dotty brewfile remove` end-to-end against the fake
// brew, pinning the Brewfile edits, the best-effort trust revocation, and the
// no-name picker guards.
func TestBrewfileRemove(t *testing.T) {
	t.Run("removes named formulae", func(t *testing.T) {
		path, _ := brewfileEnv(t, `brew "jq"`, `brew "ripgrep"`)
		if err := execDotty(t, "brewfile", "remove", "jq"); err != nil {
			t.Fatalf("remove jq: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(got), "jq") || !strings.Contains(string(got), "ripgrep") {
			t.Errorf("Brewfile after remove = %q, want ripgrep only", got)
		}
	})

	t.Run("skips names not in the Brewfile without calling bundle remove", func(t *testing.T) {
		path, logPath := brewfileEnv(t, `brew "jq"`)
		if err := execDotty(t, "brewfile", "remove", "nope"); err != nil {
			t.Fatalf("remove nope: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), `brew "jq"`) {
			t.Errorf("Brewfile after skipped remove = %q, want jq kept", got)
		}
		log, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(log), "bundle remove") {
			t.Errorf("brew calls = %q, want no bundle remove for a not-found name", log)
		}
	})

	t.Run("revokes trust for tap-qualified formulae", func(t *testing.T) {
		_, logPath := brewfileEnv(t, `brew "acme/tap/widget", trusted: true`)
		if err := execDotty(t, "brewfile", "remove", "acme/tap/widget"); err != nil {
			t.Fatalf("remove acme/tap/widget: %v", err)
		}
		log, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(log), "untrust --formula acme/tap/widget") {
			t.Errorf("brew calls = %q, want an untrust for the removed name", log)
		}
	})

	t.Run("a failed untrust warns instead of failing", func(t *testing.T) {
		path, _ := brewfileEnv(t, `brew "acme/tap/widget", trusted: true`)
		t.Setenv("DOTTY_TEST_UNTRUST_EXIT", "1")
		if err := execDotty(t, "brewfile", "remove", "acme/tap/widget"); err != nil {
			t.Fatalf("remove with failing untrust: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(got), "widget") {
			t.Errorf("Brewfile after remove = %q, want the entry gone despite the untrust failure", got)
		}
	})

	t.Run("no names and no terminal is an error", func(t *testing.T) {
		brewfileEnv(t, `brew "jq"`)
		err := execDotty(t, "brewfile", "remove")
		if err == nil || !strings.Contains(err.Error(), "no terminal") {
			t.Fatalf("remove without names = %v, want a no-terminal picker error", err)
		}
	})

	t.Run("no names and no Brewfile is a no-op", func(t *testing.T) {
		_, logPath := brewfileEnv(t)
		if err := execDotty(t, "brewfile", "remove"); err != nil {
			t.Fatalf("remove with no Brewfile: %v", err)
		}
		if calls := brewCalls(t, logPath); len(calls) != 0 {
			t.Errorf("brew calls = %v, want none for a missing Brewfile", calls)
		}
	})
}

// TestResolveBrewfilePath pins where the brewfile verbs look for the Brewfile:
// the --profile flag's profile when given (and it must exist), otherwise the
// active profile (and one must be active).
func TestResolveBrewfilePath(t *testing.T) {
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

			got, err := resolveBrewfilePath()
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("resolveBrewfilePath() error = %v, want %v", err, c.wantErr)
			}
			if c.wantErr != nil {
				return
			}
			want := profile.BrewfilePath(profile.Dir(configDir, c.wantProfile))
			if got != want {
				t.Errorf("resolveBrewfilePath() = %q, want %q", got, want)
			}
		})
	}
}

// TestBrewfileKindDispatch pins the flag-to-kind mapping the add and remove
// verbs share, and remove's --mas extension of it.
func TestBrewfileKindDispatch(t *testing.T) {
	cases := []struct {
		name  string
		flags BrewfileKindFlags
		want  brewfile.Kind
	}{
		{name: "default is formula", flags: BrewfileKindFlags{}, want: brewfile.KindFormula},
		{name: "formula", flags: BrewfileKindFlags{Formula: true}, want: brewfile.KindFormula},
		{name: "cask", flags: BrewfileKindFlags{Cask: true}, want: brewfile.KindCask},
		{name: "tap", flags: BrewfileKindFlags{Tap: true}, want: brewfile.KindTap},
		{name: "vscode", flags: BrewfileKindFlags{VSCode: true}, want: brewfile.KindVSCode},
		{name: "go", flags: BrewfileKindFlags{Go: true}, want: brewfile.KindGo},
		{name: "cargo", flags: BrewfileKindFlags{Cargo: true}, want: brewfile.KindCargo},
		{name: "uv", flags: BrewfileKindFlags{UV: true}, want: brewfile.KindUV},
		{name: "flatpak", flags: BrewfileKindFlags{Flatpak: true}, want: brewfile.KindFlatpak},
		{name: "krew", flags: BrewfileKindFlags{Krew: true}, want: brewfile.KindKrew},
		{name: "npm", flags: BrewfileKindFlags{NPM: true}, want: brewfile.KindNPM},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.flags.kind(); got != c.want {
				t.Errorf("kind() = %s, want %s", got, c.want)
			}
			// remove's kind() defers to the shared mapping when --mas is unset.
			rm := BrewfileRemoveFlags{BrewfileKindFlags: c.flags}
			if got := rm.kind(); got != c.want {
				t.Errorf("remove kind() = %s, want %s", got, c.want)
			}
		})
	}
	t.Run("mas is remove-only", func(t *testing.T) {
		rm := BrewfileRemoveFlags{MAS: true}
		if got := rm.kind(); got != brewfile.KindMAS {
			t.Errorf("remove kind() = %s, want %s", got, brewfile.KindMAS)
		}
	})
}
