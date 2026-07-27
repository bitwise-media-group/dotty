// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/profile"
)

// profileEnv points dotty at a scratch config dir holding the two profile
// shapes a machine really has: work links into a dotfiles repository the way
// dotty init leaves it, and personal is the plain directory `dotty profile
// new` makes before there is a repository. personal is active.
func profileEnv(t *testing.T) (configDir, backing string) {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	configDir = filepath.Join(xdg, "dotty")
	repo := t.TempDir()

	// Command flags are package-level, so a run leaves its values behind for
	// the next one.
	t.Cleanup(func() {
		rootFlags.Profile = ""
		profileGetFlags = ProfileGetFlags{Format: "text"}
		profileDeleteFlags = ProfileDeleteFlags{}
	})

	if _, err := profile.Create(configDir, "personal", "personal machines"); err != nil {
		t.Fatal(err)
	}
	backing = filepath.Join(repo, "profiles", "work")
	if err := os.MkdirAll(backing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile.MetadataPath(backing),
		[]byte(`{"profile":"work","description":"employer machines"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A Brewfile keeps activation from dumping one off the real machine.
	if err := os.WriteFile(profile.BrewfilePath(backing),
		[]byte("# work packages\nbrew \"jq\"\n\ncask \"ghostty\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(backing, profile.Dir(configDir, "work")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("personal", filepath.Join(configDir, "active-profile")); err != nil {
		t.Fatal(err)
	}
	return configDir, backing
}

// captureOut runs fn with os.Stdout redirected and returns what it printed.
// cli.System() reads the process streams on every call, so the swap reaches
// the command tree.
func captureOut(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	runErr := fn()
	os.Stdout = saved
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return string(out), runErr
}

// TestProfileList pins the listing: every profile, sorted, with * on the
// active one and nothing else.
func TestProfileList(t *testing.T) {
	profileEnv(t)

	out, err := captureOut(t, func() error { return execDotty(t, "profile", "ls") })
	if err != nil {
		t.Fatalf("profile ls: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want header plus two profiles:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[1], "*") || !strings.Contains(lines[1], "personal") {
		t.Errorf("active profile row = %q, want * on personal", lines[1])
	}
	if strings.HasPrefix(lines[2], "*") || !strings.Contains(lines[2], "work") {
		t.Errorf("inactive profile row = %q, want unmarked work", lines[2])
	}
	if !strings.Contains(out, "employer machines") {
		t.Errorf("descriptions missing from listing:\n%s", out)
	}
}

// TestProfileGet pins the detail view, including the state that only the
// machine knows: where the profile links to, and whether it is active.
func TestProfileGet(t *testing.T) {
	configDir, backing := profileEnv(t)

	out, err := captureOut(t, func() error { return execDotty(t, "profile", "get", "work") })
	if err != nil {
		t.Fatalf("profile get work: %v", err)
	}
	for _, want := range []string{
		"NAME         work",
		"DESCRIPTION  employer machines",
		"PATH         " + profile.Dir(configDir, "work"),
		"LINKS TO     " + backing,
		"ACTIVE       no",
		"BREWFILE     2 entries",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("get output missing %q:\n%s", want, out)
		}
	}

	// No name: the active profile, which is a plain directory with no
	// Brewfile and so has neither a link target nor entries to report.
	out, err = captureOut(t, func() error { return execDotty(t, "profile", "get") })
	if err != nil {
		t.Fatalf("profile get: %v", err)
	}
	for _, want := range []string{"NAME         personal", "ACTIVE       yes", "BREWFILE     none"} {
		if !strings.Contains(out, want) {
			t.Errorf("get output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "LINKS TO") {
		t.Errorf("plain directory reported a link target:\n%s", out)
	}
}

// TestProfileGetJSON pins that --format=json hands back profile.json as it
// sits on disk, keys the metadata struct ignores included.
func TestProfileGetJSON(t *testing.T) {
	profileEnv(t)

	out, err := captureOut(t, func() error {
		return execDotty(t, "profile", "get", "work", "--format=json")
	})
	if err != nil {
		t.Fatalf("profile get --format=json: %v", err)
	}
	if want := `{"profile":"work","description":"employer machines"}` + "\n"; out != want {
		t.Errorf("json output = %q, want %q", out, want)
	}
}

// TestProfileDelete pins the delete an issue-58 user is really asking for: the
// link goes and so does the repository directory holding the content, because
// leaving that behind means the next `dotty dotfiles link` restores it.
func TestProfileDelete(t *testing.T) {
	configDir, backing := profileEnv(t)

	if err := execDotty(t, "profile", "delete", "work"); err != nil {
		t.Fatalf("profile delete work: %v", err)
	}
	if _, err := os.Lstat(profile.Dir(configDir, "work")); !os.IsNotExist(err) {
		t.Errorf("config entry survived the delete: %v", err)
	}
	if _, err := os.Stat(backing); !os.IsNotExist(err) {
		t.Errorf("repository directory survived the delete: %v", err)
	}
	if !profile.Exists(configDir, "personal") {
		t.Error("delete took the wrong profile")
	}
}

// TestProfileDeleteActive pins the replacement rule: the active profile only
// goes once another one is activated in its place, which a non-interactive run
// has to name.
func TestProfileDeleteActive(t *testing.T) {
	configDir, _ := profileEnv(t)

	err := execDotty(t, "profile", "delete", "personal")
	if err == nil {
		t.Fatal("deleting the active profile without a replacement succeeded")
	}
	if !strings.Contains(err.Error(), "--activate") {
		t.Errorf("error = %v, want it to name --activate", err)
	}
	if !profile.Exists(configDir, "personal") {
		t.Fatal("the refused delete removed the profile anyway")
	}

	if err := execDotty(t, "profile", "delete", "personal", "--activate=work"); err != nil {
		t.Fatalf("profile delete --activate: %v", err)
	}
	if profile.Exists(configDir, "personal") {
		t.Error("personal survived the delete")
	}
	active, err := profile.ActiveName(configDir)
	if err != nil || active != "work" {
		t.Errorf("ActiveName() = %q, %v; want work", active, err)
	}
}

// TestProfileDeleteLast pins that a machine is never left with no profile.
func TestProfileDeleteLast(t *testing.T) {
	configDir, _ := profileEnv(t)

	if err := execDotty(t, "profile", "delete", "work"); err != nil {
		t.Fatalf("profile delete work: %v", err)
	}
	err := execDotty(t, "profile", "rm", "personal")
	if !errors.Is(err, profile.ErrLastProfile) {
		t.Errorf("error = %v, want ErrLastProfile", err)
	}
	if !profile.Exists(configDir, "personal") {
		t.Error("the last profile was deleted")
	}
}
