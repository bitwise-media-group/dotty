// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnvValidationErrors pins the env paths that fail before any keychain or
// stdin access, so they are safe to run end-to-end through the command tree.
func TestEnvValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantSub string
	}{
		{
			name:    "add rejects invalid key",
			args:    []string{"env", "--namespace", "test", "add", "1BAD"},
			wantSub: "invalid key",
		},
		{
			name:    "get rejects malformed bare key",
			args:    []string{"env", "--namespace", "test", "get", "1BAD"},
			wantSub: "malformed reference",
		},
		{
			name:    "get rejects bad namespace in ref",
			args:    []string{"env", "get", "dotty://a:b/KEY"},
			wantSub: "invalid namespace",
		},
		{
			name:    "run without a command errors",
			args:    []string{"env", "run", "--namespace", "test", "--"},
			wantSub: "no command given",
		},
		{
			name:    "run without args at all errors",
			args:    []string{"env", "run"},
			wantSub: "no command given",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := execDotty(t, tt.args...)
			if err == nil {
				t.Fatalf("execute %v = nil, want error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("execute %v error = %q, want substring %q", tt.args, err, tt.wantSub)
			}
		})
	}
}

// TestEnvAddCaptureErrors pins the `env add --in-file` paths that fail before
// any keychain access — argument conflicts, an unreadable or secret-free file,
// and the refusal to clobber an existing file without an interactive
// confirmation (the test binary has no terminal). The refusal must leave the
// input file untouched.
func TestEnvAddCaptureErrors(t *testing.T) {
	t.Cleanup(func() { envAddFlags = EnvAddFlags{} })

	dir := t.TempDir()
	const existingBody = "TOKEN=abc\n"
	existing := filepath.Join(dir, "exists.env")
	if err := os.WriteFile(existing, []byte(existingBody), 0o600); err != nil {
		t.Fatal(err)
	}
	noSecrets := filepath.Join(dir, "empty.env")
	if err := os.WriteFile(noSecrets, []byte("# just a comment\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		args    []string
		wantSub string
	}{
		{
			name:    "KEY cannot combine with in-file",
			args:    []string{"env", "add", "SOMEKEY", "--in-file", existing},
			wantSub: "cannot be combined",
		},
		{
			name:    "unreadable in-file errors",
			args:    []string{"env", "add", "--in-file", filepath.Join(dir, "nope.env")},
			wantSub: "read env file",
		},
		{
			name:    "file with no secrets errors",
			args:    []string{"env", "add", "--in-file", noSecrets, "--out-file", filepath.Join(dir, "out.env")},
			wantSub: "no secret values found",
		},
		{
			name:    "refuses to overwrite without a terminal",
			args:    []string{"env", "add", "--namespace", "test", "--in-file", existing},
			wantSub: "refusing to overwrite",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envAddFlags = EnvAddFlags{}
			err := execDotty(t, tt.args...)
			if err == nil {
				t.Fatalf("execute %v = nil, want error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("execute %v error = %q, want substring %q", tt.args, err, tt.wantSub)
			}
		})
	}

	// The refused overwrite must not have rewritten the input file.
	if got, err := os.ReadFile(existing); err != nil || string(got) != existingBody {
		t.Errorf("input file after refusal = %q (err %v), want %q", got, err, existingBody)
	}
}
