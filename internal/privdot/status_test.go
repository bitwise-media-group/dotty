// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// seedStore runs Store through the fake runner so repo, data dir, and
// manifest agree — the StateOK baseline every case perturbs.
func seedStore(t *testing.T, repo, profile, dataDir, rel, content string) {
	t.Helper()
	if err := os.MkdirAll(AgeDir(repo, profile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RecipientsPath(repo, profile), []byte("age1fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Store(context.Background(), &fakeRunner{}, repo, profile, dataDir, rel, []byte(content)); err != nil {
		t.Fatalf("Store: %v", err)
	}
}

func TestStatusStates(t *testing.T) {
	const profile, rel = "personal", ".ssh/known_hosts"

	cases := []struct {
		name    string
		perturb func(t *testing.T, repo, dataDir string)
		want    FileState
	}{
		{"ok", func(*testing.T, string, string) {}, StateOK},
		{"stale", func(t *testing.T, repo, _ string) {
			path := filepath.Join(HomeDir(repo, profile), rel+CipherExt)
			if err := os.WriteFile(path, []byte("CIPHER:new upstream"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, StateStale},
		{"drifted", func(t *testing.T, _, dataDir string) {
			if err := os.WriteFile(PlainPath(dataDir, profile, rel), []byte("local edit"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, StateDrifted},
		{"conflict", func(t *testing.T, repo, dataDir string) {
			path := filepath.Join(HomeDir(repo, profile), rel+CipherExt)
			if err := os.WriteFile(path, []byte("CIPHER:new upstream"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(PlainPath(dataDir, profile, rel), []byte("local edit"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, StateConflict},
		{"missing plain copy", func(t *testing.T, _, dataDir string) {
			if err := os.Remove(PlainPath(dataDir, profile, rel)); err != nil {
				t.Fatal(err)
			}
		}, StateMissing},
		{"missing manifest entry", func(t *testing.T, _, dataDir string) {
			if err := (Manifest{}).Save(ManifestPath(dataDir, profile)); err != nil {
				t.Fatal(err)
			}
		}, StateMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := seedRepo(t, nil)
			dataDir := t.TempDir()
			seedStore(t, repo, profile, dataDir, rel, "content")
			tc.perturb(t, repo, dataDir)
			statuses, err := Status(dataDir, repo, profile)
			if err != nil {
				t.Fatalf("Status: %v", err)
			}
			if len(statuses) != 1 {
				t.Fatalf("statuses = %v, want one entry", statuses)
			}
			if statuses[0].State != tc.want {
				t.Errorf("state = %s, want %s", statuses[0].State, tc.want)
			}
		})
	}
}

func TestManifestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "manifest.json")
	m := Manifest{
		".ssh/known_hosts": {CipherHash: "aa", PlainHash: "bb", DecryptedAt: time.Now().UTC()},
	}
	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if got[".ssh/known_hosts"].CipherHash != "aa" {
		t.Errorf("round-trip lost data: %+v", got)
	}
	empty, err := LoadManifest(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || len(empty) != 0 {
		t.Errorf("missing manifest should load empty: %v, %v", empty, err)
	}
}

func TestRekeyRefreshesManifest(t *testing.T) {
	const profile, rel = "personal", ".config/private/git/config"
	repo := seedRepo(t, nil)
	dataDir := t.TempDir()
	seedStore(t, repo, profile, dataDir, rel, "[user]\n")
	if err := os.WriteFile(IdentityPath(repo, profile, "123"), []byte("AGE-SECRET-KEY-1X\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := LoadManifest(ManifestPath(dataDir, profile))
	if err != nil {
		t.Fatal(err)
	}

	count, err := Rekey(context.Background(), &fakeRunner{}, repo, profile, dataDir)
	if err != nil {
		t.Fatalf("Rekey: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	statuses, err := Status(dataDir, repo, profile)
	if err != nil {
		t.Fatal(err)
	}
	if statuses[0].State != StateOK {
		t.Errorf("state after rekey = %s, want ok (manifest cipher hash refreshed)", statuses[0].State)
	}
	after, err := LoadManifest(ManifestPath(dataDir, profile))
	if err != nil {
		t.Fatal(err)
	}
	if after[rel].PlainHash != before[rel].PlainHash {
		t.Errorf("rekey must not change the plaintext hash")
	}
}
