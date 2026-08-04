// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"os"
	"strings"
	"testing"
)

func TestVerifyRepoFindings(t *testing.T) {
	repo := seedRepo(t, map[string]string{
		".ssh/known_hosts.age": "ct",   // fine: encrypted
		".ssh/known_hosts":     "oops", // plaintext sibling
		".ssh/id_ed25519_sk":   "stub", // sensitive, never encrypted
		".ssh/id_ed25519.pub":  "pub",  // public half: fine
		".zsh_history":         "hist", // sensitive by name
		".config/lsd/ok.yaml":  "ok",   // benign plaintext
	})
	if err := os.WriteFile(RecipientsPath(repo, "personal"), []byte("age1x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Recipients exist but no identity stub: half-finished enrollment.
	findings, err := VerifyRepo(repo)
	if err != nil {
		t.Fatalf("VerifyRepo: %v", err)
	}
	got := map[string]bool{}
	for _, f := range findings {
		got[f.Path] = true
		t.Logf("finding: %s: %s", f.Path, f.Problem)
	}
	wantPaths := []string{
		"profiles/personal/home/.ssh/known_hosts",
		"profiles/personal/home/.ssh/id_ed25519_sk",
		"profiles/personal/home/.zsh_history",
		"profiles/personal/age/recipients.txt",
	}
	for _, p := range wantPaths {
		if !got[p] {
			t.Errorf("missing finding for %s", p)
		}
	}
	for p := range got {
		if strings.Contains(p, "ok.yaml") || strings.Contains(p, ".pub") {
			t.Errorf("false positive on %s", p)
		}
	}
	if len(findings) != len(wantPaths) {
		t.Errorf("findings = %d, want %d", len(findings), len(wantPaths))
	}
}

func TestVerifyPathsIgnoresFurniture(t *testing.T) {
	repo := seedRepo(t, nil)
	findings := VerifyPaths(repo, []string{
		".gitignore",
		"profiles/personal/age/recipients.txt",
		"profiles/personal/home/.gitkeep",
		"profiles/personal/home/.ssh/config.d/base.conf.age",
	})
	if len(findings) != 0 {
		t.Errorf("furniture and ciphertext should not be findings: %v", findings)
	}
}
