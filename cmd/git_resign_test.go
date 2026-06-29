// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"strings"
	"testing"
)

// TestGitResignValidation pins the target selection rules, which are enforced
// before any git invocation: exactly one of --root or a <commitish>.
func TestGitResignValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantSub string
	}{
		{
			name:    "no target",
			args:    []string{"git", "resign"},
			wantSub: "specify --root or a <commitish>",
		},
		{
			name:    "root and commitish together",
			args:    []string{"git", "resign", "--root", "HEAD~1"},
			wantSub: "mutually exclusive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitResignFlags = GitResignFlags{} // pflag keeps values between parses
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
