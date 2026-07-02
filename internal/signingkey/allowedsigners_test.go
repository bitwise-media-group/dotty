// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package signingkey

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// skAlgo and skUser are the public-key algorithm and username every test key
// in this file uses.
const (
	skAlgo = "sk-ssh-ed25519@openssh.com"
	skUser = "deavon"
)

// writeStubPub writes an ed25519 skAlgo public-key sidecar with the given blob
// under dir and returns a KeyRef pointing at it. The private stub itself is not
// needed: Trust reads only the .pub.
func writeStubPub(t *testing.T, dir, serial, blob string) KeyRef {
	t.Helper()
	const typ = "ed25519"
	priv := filepath.Join(dir, serial, "id_"+typ+"_sk_"+skUser)
	if err := os.MkdirAll(filepath.Dir(priv), 0o700); err != nil {
		t.Fatal(err)
	}
	pub := priv + ".pub"
	if err := os.WriteFile(pub, []byte(skAlgo+" "+blob+" "+skUser+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return KeyRef{Serial: serial, Type: typ, User: skUser, PrivPath: priv, PubPath: pub}
}

// wantCounts fails the test unless result added and skipped exactly the given
// number of entries.
func wantCounts(t *testing.T, result Trusted, added, skipped int) {
	t.Helper()
	if len(result.Added) != added || len(result.Skipped) != skipped {
		t.Errorf("added %d skipped %d, want %d and %d", len(result.Added), len(result.Skipped), added, skipped)
	}
}

// wantPerm fails the test unless path exists with exactly perm.
func wantPerm(t *testing.T, path string, perm os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != perm {
		t.Errorf("%s perm = %o, want %o", path, got, perm)
	}
}

func TestTrust(t *testing.T) {

	t.Run("appends one entry per key and creates the file 0600", func(t *testing.T) {
		data := t.TempDir()
		a := writeStubPub(t, data, "111", "BLOBA")
		b := writeStubPub(t, data, "222", "BLOBB")
		path := filepath.Join(t.TempDir(), "ssh", "allowed_signers")

		result, err := Trust(path, "deavon@example.com", []KeyRef{a, b})
		if err != nil {
			t.Fatalf("Trust() error: %v", err)
		}
		wantCounts(t, result, 2, 0)
		got := readFile(t, path)
		want := "deavon@example.com " + skAlgo + " BLOBA\n" +
			"deavon@example.com " + skAlgo + " BLOBB\n"
		if got != want {
			t.Errorf("file =\n%q\nwant\n%q", got, want)
		}
		wantPerm(t, path, 0o600)
		wantPerm(t, filepath.Dir(path), 0o700)
	})

	t.Run("re-running is idempotent", func(t *testing.T) {
		data := t.TempDir()
		a := writeStubPub(t, data, "111", "BLOBA")
		path := filepath.Join(t.TempDir(), "allowed_signers")

		if _, err := Trust(path, "deavon@example.com", []KeyRef{a}); err != nil {
			t.Fatalf("first Trust() error: %v", err)
		}
		first := readFile(t, path)
		result, err := Trust(path, "deavon@example.com", []KeyRef{a})
		if err != nil {
			t.Fatalf("second Trust() error: %v", err)
		}
		wantCounts(t, result, 0, 1)
		if got := readFile(t, path); got != first {
			t.Errorf("file changed on re-run:\n%q\n->\n%q", first, got)
		}
	})

	t.Run("preserves existing content and fixes a missing trailing newline", func(t *testing.T) {
		data := t.TempDir()
		a := writeStubPub(t, data, "111", "BLOBA")
		path := filepath.Join(t.TempDir(), "allowed_signers")
		// A hand-written comment and an entry with no trailing newline.
		if err := os.WriteFile(path, []byte("# my signers\nother@x.y ssh-ed25519 KEEP"), 0o600); err != nil {
			t.Fatal(err)
		}

		if _, err := Trust(path, "deavon@example.com", []KeyRef{a}); err != nil {
			t.Fatalf("Trust() error: %v", err)
		}
		got := readFile(t, path)
		want := "# my signers\nother@x.y ssh-ed25519 KEEP\n" +
			"deavon@example.com " + skAlgo + " BLOBA\n"
		if got != want {
			t.Errorf("file =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("dedupes against an entry carrying options", func(t *testing.T) {
		data := t.TempDir()
		a := writeStubPub(t, data, "111", "BLOBA")
		path := filepath.Join(t.TempDir(), "allowed_signers")
		existing := `deavon@example.com namespaces="git" ` + skAlgo + " BLOBA\n"
		if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
			t.Fatal(err)
		}

		result, err := Trust(path, "deavon@example.com", []KeyRef{a})
		if err != nil {
			t.Fatalf("Trust() error: %v", err)
		}
		wantCounts(t, result, 0, 1)
		if got := readFile(t, path); got != existing {
			t.Errorf("file changed:\n%q", got)
		}
	})

	t.Run("a different email for the same key is a new entry", func(t *testing.T) {
		data := t.TempDir()
		a := writeStubPub(t, data, "111", "BLOBA")
		path := filepath.Join(t.TempDir(), "allowed_signers")
		if _, err := Trust(path, "old@example.com", []KeyRef{a}); err != nil {
			t.Fatal(err)
		}
		result, err := Trust(path, "new@example.com", []KeyRef{a})
		if err != nil {
			t.Fatalf("Trust() error: %v", err)
		}
		if len(result.Added) != 1 {
			t.Errorf("added %d, want 1 (new principal is a distinct entry)", len(result.Added))
		}
	})

	t.Run("a malformed public key aborts without writing", func(t *testing.T) {
		data := t.TempDir()
		bad := KeyRef{Serial: "111", Type: "ed25519", User: "deavon",
			PubPath: filepath.Join(data, "bad.pub")}
		if err := os.WriteFile(bad.PubPath, []byte("garbage\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "allowed_signers")

		if _, err := Trust(path, "deavon@example.com", []KeyRef{bad}); err == nil {
			t.Fatal("Trust() error = nil, want malformed-key error")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("file was written despite the error: %v", err)
		}
	})
}

func TestParseAllowedSigner(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		wantPrincipal string
		wantID        string
		wantOK        bool
	}{
		{
			name:          "plain entry",
			line:          "a@b.c ssh-ed25519 BLOB",
			wantPrincipal: "a@b.c",
			wantID:        "ssh-ed25519 BLOB",
			wantOK:        true,
		},
		{
			name:          "sk entry with comment",
			line:          "a@b.c sk-ssh-ed25519@openssh.com BLOB tail",
			wantPrincipal: "a@b.c",
			wantID:        "sk-ssh-ed25519@openssh.com BLOB",
			wantOK:        true,
		},
		{
			name:          "entry with options",
			line:          `a@b.c namespaces="git" ecdsa-sha2-nistp256 BLOB`,
			wantPrincipal: "a@b.c",
			wantID:        "ecdsa-sha2-nistp256 BLOB",
			wantOK:        true,
		},
		{name: "comment line", line: "# a comment", wantOK: false},
		{name: "blank line", line: "   ", wantOK: false},
		{name: "no key blob", line: "a@b.c ssh-ed25519", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			principal, id, ok := parseAllowedSigner(tt.line)
			if ok != tt.wantOK || principal != tt.wantPrincipal || id != tt.wantID {
				t.Errorf("parseAllowedSigner(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.line, principal, id, ok, tt.wantPrincipal, tt.wantID, tt.wantOK)
			}
		})
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}
