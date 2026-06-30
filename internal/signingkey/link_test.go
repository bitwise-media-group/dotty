// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package signingkey

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultLinkPath(t *testing.T) {
	got := DefaultLinkPath("/home/alice")
	want := filepath.Join("/home/alice", ".ssh", "id_sk_current")
	if got != want {
		t.Errorf("DefaultLinkPath() = %q, want %q", got, want)
	}
}

func TestLink(t *testing.T) {
	dataDir := t.TempDir()
	ed := writeKey(t, dataDir, "111", "ed25519", "alice", "sk-ssh-ed25519@openssh.com AAAA1 alice")

	// A nested link path is created, with both the stub and its .pub linked.
	link := filepath.Join(t.TempDir(), "ssh", "current")
	if err := Link(link, ed); err != nil {
		t.Fatalf("Link() error: %v", err)
	}
	if got, _ := os.Readlink(link); got != ed.PrivPath {
		t.Errorf("stub link = %q, want %q", got, ed.PrivPath)
	}
	if got, _ := os.Readlink(link + ".pub"); got != ed.PubPath {
		t.Errorf("pub link = %q, want %q", got, ed.PubPath)
	}
	// The link resolves to the stub's contents.
	if priv, err := os.ReadFile(link); err != nil || len(priv) == 0 {
		t.Errorf("read via link: %v (len %d)", err, len(priv))
	}

	// Re-linking atomically repoints at another key — the use case when a
	// different YubiKey is plugged in.
	other := writeKey(t, dataDir, "222", "ed25519", "bob", "sk-ssh-ed25519@openssh.com AAAA2 bob")
	if err := Link(link, other); err != nil {
		t.Fatalf("relink error: %v", err)
	}
	if got, _ := os.Readlink(link); got != other.PrivPath {
		t.Errorf("relinked stub = %q, want %q", got, other.PrivPath)
	}
	if got, _ := os.Readlink(link + ".pub"); got != other.PubPath {
		t.Errorf("relinked pub = %q, want %q", got, other.PubPath)
	}
}
