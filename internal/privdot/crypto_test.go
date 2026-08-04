// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package privdot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// fakeRunner records invocations and plays canned crypto: encrypt wraps the
// stdin behind a nonce marker — different ciphertext every call, like real
// age — and decrypt unwraps it, so Store/Rekey round-trips are testable
// without age.
type fakeRunner struct {
	calls [][]string
	err   error
	// missing simulates age absent from PATH.
	missing bool
	nonce   int
}

func (r *fakeRunner) OutputStdin(_ context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.err != nil {
		return nil, r.err
	}
	if args[0] == "--encrypt" {
		r.nonce++
		return append(fmt.Appendf(nil, "CIPHER%d:", r.nonce), stdin...), nil
	}
	_, pt, ok := bytes.Cut(stdin, []byte(":"))
	if !ok {
		return nil, errors.New("fake decrypt: not fake ciphertext")
	}
	return pt, nil
}

func (r *fakeRunner) LookPath(name string) (string, error) {
	if r.missing {
		return "", errors.New(name + " not found in PATH")
	}
	return "/fake/" + name, nil
}

func TestEncryptArgs(t *testing.T) {
	r := &fakeRunner{}
	ct, err := Encrypt(context.Background(), r, "/repo/age/recipients.txt", []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	want := []string{"age", "--encrypt", "-R", "/repo/age/recipients.txt", "-a"}
	if !reflect.DeepEqual(r.calls[0], want) {
		t.Errorf("argv = %v, want %v", r.calls[0], want)
	}
	if string(ct) != "CIPHER1:secret" {
		t.Errorf("ciphertext = %q", ct)
	}
}

func TestEncryptMissingAge(t *testing.T) {
	r := &fakeRunner{missing: true}
	if _, err := Encrypt(context.Background(), r, "r.txt", nil); err == nil {
		t.Fatal("Encrypt with no age binary should fail")
	}
	if len(r.calls) != 0 {
		t.Errorf("no exec should happen when age is missing, got %v", r.calls)
	}
}

func TestDecryptArgs(t *testing.T) {
	r := &fakeRunner{}
	pt, err := Decrypt(context.Background(), r, []string{"/a/id-1.txt", "/a/id-2.txt"},
		[]byte("CIPHER1:secret"))
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	want := []string{"age", "--decrypt", "-i", "/a/id-1.txt", "-i", "/a/id-2.txt"}
	if !reflect.DeepEqual(r.calls[0], want) {
		t.Errorf("argv = %v, want %v", r.calls[0], want)
	}
	if string(pt) != "secret" {
		t.Errorf("plaintext = %q", pt)
	}
}

func TestDecryptNoIdentities(t *testing.T) {
	if _, err := Decrypt(context.Background(), &fakeRunner{}, nil, []byte("ct")); err == nil {
		t.Fatal("Decrypt with no identities should fail")
	}
}

func TestRequireTerminal(t *testing.T) {
	dir := t.TempDir()
	software := filepath.Join(dir, "identity-soft.txt")
	if err := os.WriteFile(software, []byte("AGE-SECRET-KEY-1XYZ\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(dir, "identity-123.txt")
	if err := os.WriteFile(plugin, []byte("#serial: 123\nAGE-PLUGIN-YUBIKEY-1ABC\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Buffers are not terminals, so these streams read as non-interactive.
	ios := cli.IOStreams{In: &bytes.Buffer{}, Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}

	if err := RequireTerminal(ios, []string{software}); err != nil {
		t.Errorf("software identity should not need a terminal: %v", err)
	}
	if err := RequireTerminal(ios, []string{software, plugin}); !errors.Is(err, ErrNeedsTerminal) {
		t.Errorf("plugin identity without a terminal: err = %v, want ErrNeedsTerminal", err)
	}
}
