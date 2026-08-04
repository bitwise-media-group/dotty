// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// quietIOS returns non-terminal streams capturing warnings.
func quietIOS() (cli.IOStreams, *bytes.Buffer) {
	var errOut bytes.Buffer
	return cli.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &errOut}, &errOut
}

// seedEncrypted plants fake ciphertext and a software identity for the test
// profile.
func seedEncrypted(t *testing.T, repo string, entries map[string]string) {
	t.Helper()
	for rel, pt := range entries {
		path := filepath.Join(HomeDir(repo, testProfile), rel+CipherExt)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("CIPHER0:"+pt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(RecipientsPath(repo, testProfile), []byte("age1x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(IdentityPath(repo, testProfile, "soft"), []byte("AGE-SECRET-KEY-1X\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeIncremental(t *testing.T) {
	const profile = "personal"
	repo := seedRepo(t, map[string]string{".ssh/config.d/base.conf": "plain contents"})
	seedEncrypted(t, repo, map[string]string{".ssh/known_hosts": "hosts"})
	dataDir := t.TempDir()
	ios, _ := quietIOS()
	ctx := context.Background()

	r := &fakeRunner{}
	rep, err := Materialize(ctx, ios, r, repo, profile, dataDir, true)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if rep.Decrypted != 1 || rep.Copied != 1 || rep.UpToDate != 0 {
		t.Errorf("first run = %+v, want 1 decrypted, 1 copied", rep)
	}
	got, err := os.ReadFile(PlainPath(dataDir, profile, ".ssh/known_hosts"))
	if err != nil || string(got) != "hosts" {
		t.Errorf("decrypted content = %q, %v", got, err)
	}

	// Second run: nothing changed, so no decrypt call may happen at all.
	r = &fakeRunner{}
	rep, err = Materialize(ctx, ios, r, repo, profile, dataDir, true)
	if err != nil {
		t.Fatalf("Materialize rerun: %v", err)
	}
	if rep.UpToDate != 2 || rep.Decrypted != 0 || rep.Copied != 0 {
		t.Errorf("rerun = %+v, want 2 up-to-date", rep)
	}
	if len(r.calls) != 0 {
		t.Errorf("steady-state rerun must not exec anything, got %v", r.calls)
	}
}

func TestMaterializeConflictNeverClobbers(t *testing.T) {
	const profile, rel = "personal", ".config/private/git/config"
	repo := seedRepo(t, nil)
	seedEncrypted(t, repo, map[string]string{rel: "upstream v1"})
	dataDir := t.TempDir()
	ios, errOut := quietIOS()
	ctx := context.Background()

	if _, err := Materialize(ctx, ios, &fakeRunner{}, repo, profile, dataDir, true); err != nil {
		t.Fatal(err)
	}
	// Local edit plus upstream change: both sides moved.
	if err := os.WriteFile(PlainPath(dataDir, profile, rel), []byte("local edit"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctPath := filepath.Join(HomeDir(repo, profile), rel+CipherExt)
	if err := os.WriteFile(ctPath, []byte("CIPHER0:upstream v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Materialize(ctx, ios, &fakeRunner{}, repo, profile, dataDir, false)
	if err != nil {
		t.Fatalf("non-strict conflict must not fail: %v", err)
	}
	if len(rep.Skipped) != 1 {
		t.Errorf("conflict should be skipped, got %+v", rep)
	}
	got, _ := os.ReadFile(PlainPath(dataDir, profile, rel))
	if string(got) != "local edit" {
		t.Errorf("local edit was clobbered: %q", got)
	}
	if !strings.Contains(errOut.String(), "conflict") {
		t.Errorf("expected a conflict warning, got %q", errOut.String())
	}
	if _, err := Materialize(ctx, ios, &fakeRunner{}, repo, profile, dataDir, true); err == nil {
		t.Error("strict conflict should fail")
	}
}

func TestMaterializeSkipsUndecryptable(t *testing.T) {
	const profile, rel = "personal", ".ssh/known_hosts"
	repo := seedRepo(t, nil)
	seedEncrypted(t, repo, map[string]string{rel: "hosts"})
	dataDir := t.TempDir()
	ios, errOut := quietIOS()

	r := &fakeRunner{err: os.ErrPermission}
	rep, err := Materialize(context.Background(), ios, r, repo, profile, dataDir, false)
	if err != nil {
		t.Fatalf("non-strict decrypt failure must not fail the run: %v", err)
	}
	if len(rep.Skipped) != 1 || rep.Decrypted != 0 {
		t.Errorf("rep = %+v, want one skip", rep)
	}
	if !strings.Contains(errOut.String(), "plugged in") {
		t.Errorf("warning should hint at the unplugged key, got %q", errOut.String())
	}
}

func TestMaterializePrunesRemovedEntries(t *testing.T) {
	const profile = "personal"
	repo := seedRepo(t, nil)
	seedEncrypted(t, repo, map[string]string{".ssh/known_hosts": "hosts", ".ssh/extra": "gone soon"})
	dataDir := t.TempDir()
	ios, _ := quietIOS()
	ctx := context.Background()

	if _, err := Materialize(ctx, ios, &fakeRunner{}, repo, profile, dataDir, true); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(HomeDir(repo, profile), ".ssh/extra"+CipherExt)); err != nil {
		t.Fatal(err)
	}
	rep, err := Materialize(ctx, ios, &fakeRunner{}, repo, profile, dataDir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Pruned) != 1 || rep.Pruned[0] != ".ssh/extra" {
		t.Errorf("pruned = %v, want [.ssh/extra]", rep.Pruned)
	}
	if _, err := os.Stat(PlainPath(dataDir, profile, ".ssh/extra")); !os.IsNotExist(err) {
		t.Error("pruned plaintext should be removed")
	}
}

func TestActivateRetargets(t *testing.T) {
	dataDir := t.TempDir()
	if err := Activate(dataDir, "personal"); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if target, err := os.Readlink(ActiveLink(dataDir)); err != nil || target != "personal" {
		t.Fatalf("link = %q, %v", target, err)
	}
	if err := Activate(dataDir, "personal"); err != nil {
		t.Fatalf("re-Activate same: %v", err)
	}
	if err := Activate(dataDir, "work"); err != nil {
		t.Fatalf("retarget: %v", err)
	}
	if target, _ := os.Readlink(ActiveLink(dataDir)); target != "work" {
		t.Errorf("link = %q, want work", target)
	}
}

func TestStaleRels(t *testing.T) {
	repo := seedRepo(t, map[string]string{
		".ssh/known_hosts.age": "ct",
		".shared.conf":         "x",
	})
	if err := Scaffold(repo, "work"); err != nil {
		t.Fatal(err)
	}
	workFile := filepath.Join(HomeDir(repo, "work"), ".shared.conf")
	if err := os.WriteFile(workFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err := StaleRels(repo, "work")
	if err != nil {
		t.Fatalf("StaleRels: %v", err)
	}
	if len(stale) != 1 || stale[0] != ".ssh/known_hosts" {
		t.Errorf("stale = %v, want [.ssh/known_hosts] (shared entry must not be pruned)", stale)
	}
}
