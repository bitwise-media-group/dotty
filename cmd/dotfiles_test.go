// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/mise"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/scaffold"
	"github.com/bitwise-media-group/dotty/internal/wizard"
)

func TestDotfilesProxy(t *testing.T) {
	dir, log := packagesEnv(t, "", nil)
	for _, args := range [][]string{
		{"dotfiles", "track", "--os", "linux", "--profile", "server", "--dry-run", "~/a file"},
		{"dotfiles", "history", "show", "abc123", "--path", "~/a file"},
		{"dotfiles", "restore", "~/a file", "--dry-run"},
		{"dotfiles", "add", "--changed", "--help"},
	} {
		if err := execDotty(t, args...); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		"dot --cd " + dir + " track --os linux --profile server --dry-run ~/a file",
		"dot --cd " + dir + " history show abc123 --path ~/a file",
		"dot --cd " + dir + " rollback ~/a file --dry-run",
		"dot --cd " + dir + " add --changed --help",
	}
	if got := miseCalls(t, log, dir); !slices.Equal(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}

	configDir := filepath.Dir(filepath.Dir(dir))
	if _, err := profile.Create(configDir, "work", ""); err != nil {
		t.Fatal(err)
	}
	other := mise.Dir(profile.Dir(configDir, "work"))
	if err := mise.EnsureConfig(other); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	if err := execDotty(t, "--profile=work", "dotfiles", "status", "--json"); err != nil {
		t.Fatal(err)
	}
	if got := miseCalls(t, log, other); !slices.Equal(got, []string{"dot --cd " + other + " status --json"}) {
		t.Fatalf("selected profile calls = %v", got)
	}
}

func TestDotfilesGrokCopiesAndCapture(t *testing.T) {
	initFlags = wizard.Flags{}
	t.Cleanup(func() { initFlags = wizard.Flags{}; profileActivateFlags = ProfileActivateFlags{} })
	home := initEnv(t)
	repo := filepath.Join(home, "Repos", "dotfiles")
	grok := filepath.Join(home, ".config", "grok")
	if err := os.MkdirAll(grok, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(grok, "config.toml"), []byte("original grok config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execDotty(t, "init", "--repo="+repo, "--repos-dir="+filepath.Join(home, "Repos"),
		"--profile-name=box", "--addons=tmux", "--agents=grok", "--import-packages=false",
		"--yes", "--skip-font", "--skip-git", "--skip-install"); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(filepath.Join(home, ".local", "share", "dotty", "backups", "*", grok, "config.toml"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("grok backup = %v, %v", backups, err)
	}
	if got, err := os.ReadFile(backups[0]); err != nil || string(got) != "original grok config\n" {
		t.Fatalf("original grok config lost: %q, %v", got, err)
	}
	for _, rel := range []string{"AGENTS.md", "config.toml", "sandbox.toml", "hooks/pre-tool-use-policy", "hooks/tmux-status.json"} {
		info, err := os.Lstat(filepath.Join(grok, rel))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("grok %s is not a regular file: %v, %v", rel, info, err)
		}
	}
	if _, err := os.Readlink(filepath.Join(home, ".config", "tmux")); err != nil {
		t.Fatalf("default deployment is not a symlink: %v", err)
	}

	// Migrate old symlinks, including one inside the copied hooks tree.
	for _, rel := range []string{"AGENTS.md", "hooks/pre-tool-use-policy"} {
		target := filepath.Join(grok, rel)
		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(repo, "home", ".config", "grok", rel), target); err != nil {
			t.Fatal(err)
		}
	}
	if err := execDotty(t, "dotfiles", "link", "--on-conflict=fail"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"AGENTS.md", "hooks/pre-tool-use-policy"} {
		info, err := os.Lstat(filepath.Join(grok, rel))
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("migration left a symlink at %s: %v", rel, err)
		}
	}

	target := filepath.Join(grok, "AGENTS.md")
	if err := os.WriteFile(target, []byte("edited live\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := execDotty(t, "dotfiles", "add", "--changed", "--yes"); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(repo, "home", ".config", "grok", "AGENTS.md")
	if got, err := os.ReadFile(source); err != nil || string(got) != "edited live\n" {
		t.Fatalf("captured source = %q, %v", got, err)
	}

	// New captures must land in this repository, not mise's default ~/.dotfiles.
	newFile := filepath.Join(home, ".newconfig")
	if err := os.WriteFile(newFile, []byte("new config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := execDotty(t, "dotfiles", "add", newFile, "--yes"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(repo, "home", ".newconfig")); err != nil || !strings.Contains(string(got), "new config") {
		t.Fatalf("capture root = %q, %v", got, err)
	}
	if _, err := os.Readlink(newFile); err != nil {
		t.Fatalf("new file default mode is not symlink: %v", err)
	}
	if got, err := os.ReadFile(newFile); err != nil || string(got) != "new config\n" {
		t.Fatalf("captured symlink does not resolve: %q, %v", got, err)
	}
	if err := os.RemoveAll(grok); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(repo, "home", ".config", "grok"), grok); err != nil {
		t.Fatal(err)
	}
	if err := execDotty(t, "dotfiles", "link", "--on-conflict=fail"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(grok); err != nil || !info.IsDir() {
		t.Fatalf("grok root remains linked: %v, %v", info, err)
	}

	configDir := filepath.Join(home, ".config", "dotty")
	if _, err := profile.Create(configDir, "work", ""); err != nil {
		t.Fatal(err)
	}
	work := profile.Dir(configDir, "work")
	answers, err := scaffold.LoadAnswers(profile.Dir(configDir, "box"))
	if err != nil {
		t.Fatal(err)
	}
	answers.ProfileName = "work"
	if err := scaffold.SaveAnswers(work, answers); err != nil {
		t.Fatal(err)
	}
	workGrok := filepath.Join(work, "home", ".config", "grok")
	if err := os.MkdirAll(workGrok, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"config.toml", "sandbox.toml"} {
		if err := os.WriteFile(filepath.Join(workGrok, rel), []byte("work profile\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := mise.EnsureConfig(mise.Dir(work)); err != nil {
		t.Fatal(err)
	}
	config := "[dotfiles]\n\"~/.config/grok/config.toml\" = { source = \"../home/.config/grok/config.toml\", mode = \"copy\" }\n\"~/.config/grok/sandbox.toml\" = { source = \"../home/.config/grok/sandbox.toml\", mode = \"copy\" }\n"
	if err := os.WriteFile(mise.ConfigPath(mise.Dir(work)), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := execDotty(t, "profile", "activate", "--name=work"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(grok, "config.toml")); err != nil || string(got) != "work profile\n" {
		t.Fatalf("activated copy = %q, %v", got, err)
	}
}
