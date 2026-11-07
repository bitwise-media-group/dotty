// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package linker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

// output returns everything written to migrateIOS's buffers.
func output(ios cli.IOStreams) string {
	return ios.Out.(*bytes.Buffer).String() + ios.ErrOut.(*bytes.Buffer).String()
}

// fakeSudo records each elevated command and performs it without sudo, so
// the tests exercise the real link against a scratch site.
type fakeSudo struct {
	calls [][]string
	err   error
}

func (f *fakeSudo) RunInteractive(_ context.Context, name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	if f.err != nil {
		return f.err
	}
	switch args[0] {
	case "mkdir":
		return os.MkdirAll(args[len(args)-1], 0o755)
	case "ln":
		return os.Symlink(args[len(args)-2], args[len(args)-1])
	}
	return fmt.Errorf("unexpected command %v", args)
}

// systemEnv builds a fake home whose active profile renders the codex
// settings layer, returning the home, dotty config dir, the rendered layer
// the link must target, and a scratch system site.
func systemEnv(t *testing.T) (home, configDir, layer, site string) {
	t.Helper()
	home, configDir, _ = migrateEnv(t)
	profileDir := filepath.Join(configDir, "box")
	layer = filepath.Join(configDir, "active-profile", "home", ".config", "codex", "dotty.config.toml")
	if err := os.MkdirAll(filepath.Join(profileDir, "home", ".config", "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "home", ".config", "codex", "dotty.config.toml"),
		[]byte("sandbox_mode = \"workspace-write\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("box", filepath.Join(configDir, "active-profile")); err != nil {
		t.Fatal(err)
	}
	return home, configDir, layer, filepath.Join(home, "etc", "codex", "config.toml")
}

func TestLinkCodexSystemConfigCreatesLink(t *testing.T) {
	home, configDir, layer, site := systemEnv(t)
	sudo := &fakeSudo{}
	if err := LinkCodexSystemConfig(context.Background(), migrateIOS(), sudo, site, home, configDir); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"sudo", "mkdir", "-p", filepath.Dir(site)},
		{"sudo", "ln", "-s", layer, site},
	}
	if !slices.EqualFunc(sudo.calls, want, slices.Equal) {
		t.Errorf("calls = %v, want %v", sudo.calls, want)
	}
	if got, err := os.ReadFile(site); err != nil || !strings.Contains(string(got), "workspace-write") {
		t.Errorf("site does not resolve to the layer: %q, %v", got, err)
	}

	// A second run finds the link in place and needs no sudo.
	sudo.calls = nil
	if err := LinkCodexSystemConfig(context.Background(), migrateIOS(), sudo, site, home, configDir); err != nil {
		t.Fatal(err)
	}
	if len(sudo.calls) != 0 {
		t.Errorf("re-run ran %v, want nothing", sudo.calls)
	}
}

func TestLinkCodexSystemConfigWithoutCodex(t *testing.T) {
	home, configDir, _ := migrateEnv(t)
	site := filepath.Join(home, "etc", "codex", "config.toml")
	sudo := &fakeSudo{}
	if err := LinkCodexSystemConfig(context.Background(), migrateIOS(), sudo, site, home, configDir); err != nil {
		t.Fatal(err)
	}
	if len(sudo.calls) != 0 {
		t.Errorf("calls = %v, want none for a profile without codex", sudo.calls)
	}
	if _, err := os.Lstat(site); err == nil {
		t.Error("site created for a profile without codex")
	}
}

func TestLinkCodexSystemConfigLeavesForeignSite(t *testing.T) {
	for name, seed := range map[string]func(site string) error{
		"regular file":    func(site string) error { return os.WriteFile(site, []byte("model = \"x\"\n"), 0o644) },
		"foreign symlink": func(site string) error { return os.Symlink("/elsewhere/config.toml", site) },
	} {
		t.Run(name, func(t *testing.T) {
			home, configDir, _, site := systemEnv(t)
			if err := os.MkdirAll(filepath.Dir(site), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := seed(site); err != nil {
				t.Fatal(err)
			}
			before, _ := os.Lstat(site)
			ios := migrateIOS()
			sudo := &fakeSudo{}
			if err := LinkCodexSystemConfig(context.Background(), ios, sudo, site, home, configDir); err != nil {
				t.Fatal(err)
			}
			if len(sudo.calls) != 0 {
				t.Errorf("calls = %v, want none", sudo.calls)
			}
			if after, err := os.Lstat(site); err != nil || after.Mode() != before.Mode() {
				t.Errorf("foreign site changed: %v, %v", after, err)
			}
			if out := output(ios); !strings.Contains(out, "sudo ln -sfn") {
				t.Errorf("no replacement hint in output: %q", out)
			}
		})
	}
}

func TestLinkCodexSystemConfigSudoFails(t *testing.T) {
	home, configDir, _, site := systemEnv(t)
	sudo := &fakeSudo{err: errors.New("declined")}
	err := LinkCodexSystemConfig(context.Background(), migrateIOS(), sudo, site, home, configDir)
	if err == nil || !strings.Contains(err.Error(), "sudo mkdir") {
		t.Errorf("err = %v, want the failing sudo command", err)
	}
}

func TestLinkCodexSystemConfigWarnsOnShadowedKeys(t *testing.T) {
	home, configDir, _, site := systemEnv(t)
	machine := filepath.Join(home, ".config", "codex", "config.toml")
	if err := os.WriteFile(machine, []byte("sandbox_mode = \"danger-full-access\"\n"+codexState), 0o644); err != nil {
		t.Fatal(err)
	}
	ios := migrateIOS()
	if err := LinkCodexSystemConfig(context.Background(), ios, &fakeSudo{}, site, home, configDir); err != nil {
		t.Fatal(err)
	}
	if out := output(ios); !strings.Contains(out, "sandbox_mode") {
		t.Errorf("no shadowing warning in output: %q", out)
	}
}

func TestCodexShadowedKeys(t *testing.T) {
	tests := []struct {
		name, content string
		want          []string
	}{
		{"machine state only", "model = \"gpt\"\n" + codexState, nil},
		{"top-level keys", "approval_policy = \"never\"\nsandbox_mode = \"danger-full-access\"\n",
			[]string{"approval_policy", "sandbox_mode"}},
		{"table", "[sandbox_workspace_write]\nnetwork_access = true\n", []string{"sandbox_workspace_write"}},
		{"dotted key", "sandbox_workspace_write.network_access = true\n", []string{"sandbox_workspace_write"}},
		{"key inside another table", "[profiles.x]\nsandbox_mode = \"read-only\"\n", nil},
		{"comment", "# sandbox_mode = \"read-only\"\n", nil},
		{"array of tables", "[[sandbox_workspace_write]]\n", []string{"sandbox_workspace_write"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := codexShadowedKeys(path)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("codexShadowedKeys = %v, want %v", got, tt.want)
			}
		})
	}

	got, err := codexShadowedKeys(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil || got != nil {
		t.Errorf("missing file = %v, %v, want nil, nil", got, err)
	}
}
