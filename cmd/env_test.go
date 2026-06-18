// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
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
