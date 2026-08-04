// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// TestAgeRoundTrip drives the real age binary with a software identity —
// the same code path the YubiKey plugin rides, minus the hardware. Skipped
// when age is not on PATH.
func TestAgeRoundTrip(t *testing.T) {
	for _, bin := range []string{"age", "age-keygen"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	ctx := context.Background()
	runner := cli.NewExecRunner(cli.IOStreams{In: strings.NewReader(""), Out: os.Stderr, ErrOut: os.Stderr},
		slog.New(slog.DiscardHandler))

	const profile = "personal"
	repo := seedRepo(t, nil)
	dataDir := t.TempDir()

	// Enroll a software identity: age-keygen writes the secret key and
	// prints the recipient on stderr comment lines; -y derives it cleanly.
	idPath := IdentityPath(repo, profile, "software")
	out, err := runner.Output(ctx, "age-keygen", "-o", idPath)
	_ = out
	if err != nil {
		t.Fatalf("age-keygen: %v", err)
	}
	recipient, err := runner.Output(ctx, "age-keygen", "-y", idPath)
	if err != nil {
		t.Fatalf("derive recipient: %v", err)
	}
	if err := os.WriteFile(RecipientsPath(repo, profile), recipient, 0o644); err != nil {
		t.Fatal(err)
	}

	const rel = ".config/private/git/config"
	secret := []byte("[user]\n\tname = Private Person\n")
	if err := Store(ctx, runner, repo, profile, dataDir, rel, secret); err != nil {
		t.Fatalf("Store: %v", err)
	}

	ctPath := filepath.Join(HomeDir(repo, profile), rel+CipherExt)
	ct, err := os.ReadFile(ctPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ct), "Private Person") {
		t.Fatal("ciphertext leaks plaintext")
	}
	ids, err := Identities(repo, profile)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := Decrypt(ctx, runner, ids, ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(pt) != string(secret) {
		t.Errorf("round-trip = %q, want %q", pt, secret)
	}

	statuses, err := Status(dataDir, repo, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 1 || statuses[0].State != StateOK {
		t.Errorf("status after store = %+v, want one ok entry", statuses)
	}

	// Rekey with the same recipients still round-trips and stays ok.
	if _, err := Rekey(ctx, runner, repo, profile, dataDir); err != nil {
		t.Fatalf("Rekey: %v", err)
	}
	statuses, err = Status(dataDir, repo, profile)
	if err != nil {
		t.Fatal(err)
	}
	if statuses[0].State != StateOK {
		t.Errorf("status after rekey = %s, want ok", statuses[0].State)
	}
}
