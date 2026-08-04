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
	"github.com/bitwise-media-group/dotty/internal/privdot"
)

// seedPrivateData plants a decrypted private tree for one profile and
// activates it.
func seedPrivateData(t *testing.T, dataDir, profile string, entries map[string]string) {
	t.Helper()
	for rel, content := range entries {
		if err := privdot.WritePlain(dataDir, profile, rel, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := privdot.Activate(dataDir, profile); err != nil {
		t.Fatal(err)
	}
}

func testIOS() cli.IOStreams {
	return cli.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
}

func TestLinkPrivateRoutesThroughActiveProfile(t *testing.T) {
	dataDir := t.TempDir()
	home := t.TempDir()
	seedPrivateData(t, dataDir, "personal", map[string]string{
		".config/private/git/config": "[user]\n\tname = Personal\n",
	})
	seedPrivateData(t, dataDir, "work", map[string]string{
		".config/private/git/config": "[user]\n\tname = Work\n",
	})
	if err := privdot.Activate(dataDir, "personal"); err != nil {
		t.Fatal(err)
	}
	// Unfold ~/.config so the file itself gets linked rather than the
	// directory folding, mirroring a linked machine.
	if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}

	report, _, err := LinkPrivate(testIOS(), dataDir, home, "fail")
	if err != nil {
		t.Fatalf("LinkPrivate: %v", err)
	}
	if len(report.Linked) == 0 {
		t.Fatalf("nothing linked: %+v", report)
	}

	site := filepath.Join(home, ".config", "private")
	dest, err := os.Readlink(site)
	if err != nil {
		t.Fatalf("site is not a symlink: %v", err)
	}
	if !strings.Contains(dest, filepath.Join("private", "active-profile")) {
		t.Errorf("link %s -> %s does not route through active-profile", site, dest)
	}

	read := func() string {
		data, err := os.ReadFile(filepath.Join(home, ".config", "private", "git", "config"))
		if err != nil {
			t.Fatalf("read through link: %v", err)
		}
		return string(data)
	}
	if got := read(); !strings.Contains(got, "Personal") {
		t.Errorf("content = %q, want the personal identity", got)
	}

	// Retargeting the one data-dir symlink swaps the identity with no
	// relinking — the whole point of the indirection.
	if err := privdot.Activate(dataDir, "work"); err != nil {
		t.Fatal(err)
	}
	if got := read(); !strings.Contains(got, "Work") {
		t.Errorf("content after activate = %q, want the work identity", got)
	}

	// Relinking after the swap is a no-op: the link target string is
	// unchanged, so every site reads as already correct.
	report, _, err = LinkPrivate(testIOS(), dataDir, home, "fail")
	if err != nil {
		t.Fatalf("relink: %v", err)
	}
	if len(report.Linked)+len(report.Replaced) != 0 || report.OK == 0 {
		t.Errorf("relink should be all-OK, got %+v", report)
	}
}

func TestLinkPrivateBacksUpConflicts(t *testing.T) {
	dataDir := t.TempDir()
	home := t.TempDir()
	seedPrivateData(t, dataDir, "personal", map[string]string{".ssh/known_hosts": "managed"})

	// A real file occupies the site — the stow-era state being migrated.
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	site := filepath.Join(home, ".ssh", "known_hosts")
	if err := os.WriteFile(site, []byte("pre-existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, backupDir, err := LinkPrivate(testIOS(), dataDir, home, "backup")
	if err != nil {
		t.Fatalf("LinkPrivate: %v", err)
	}
	if len(report.Backed) != 1 {
		t.Fatalf("report = %+v, want one backup", report)
	}
	backed, err := os.ReadFile(filepath.Join(backupDir, strings.TrimPrefix(site, string(filepath.Separator))))
	if err != nil || string(backed) != "pre-existing" {
		t.Errorf("backup content = %q, %v", backed, err)
	}
	got, err := os.ReadFile(site)
	if err != nil || string(got) != "managed" {
		t.Errorf("linked content = %q, %v", got, err)
	}
}

func TestLinkPrivateNoTreeIsNoop(t *testing.T) {
	report, backupDir, err := LinkPrivate(testIOS(), t.TempDir(), t.TempDir(), "fail")
	if err != nil {
		t.Fatalf("LinkPrivate with no private tree: %v", err)
	}
	if backupDir != "" || len(report.Linked) != 0 {
		t.Errorf("expected a clean no-op, got %+v (%s)", report, backupDir)
	}
}
