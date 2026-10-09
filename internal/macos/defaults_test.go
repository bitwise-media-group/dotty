// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package macos

import (
	"context"
	"slices"
	"testing"
)

// fakeRunner records commands instead of running them.
type fakeRunner struct {
	commands [][]string
}

func (f *fakeRunner) RunInteractive(_ context.Context, name string, args ...string) error {
	f.commands = append(f.commands, append([]string{name}, args...))
	return nil
}

func TestEnforcePIVUsesSudoInteractively(t *testing.T) {
	r := &fakeRunner{}
	if err := EnforcePIV(context.Background(), r); err != nil {
		t.Fatalf("EnforcePIV: %v", err)
	}
	if len(r.commands) != 2 || r.commands[0][0] != "sudo" || r.commands[1][0] != "sudo" {
		t.Fatalf("commands = %v, want two sudo invocations", r.commands)
	}
	if !slices.Contains(r.commands[0], "enforceSmartCard") {
		t.Errorf("first command misses enforceSmartCard: %v", r.commands[0])
	}
}

func TestGroupIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range Groups {
		if seen[g.ID] {
			t.Errorf("duplicate group %q", g.ID)
		}
		seen[g.ID] = true
	}
}

func TestNeedsLogout(t *testing.T) {
	for _, tc := range []struct {
		groups []string
		want   bool
	}{
		{nil, false},
		{[]string{"dock", "finder"}, false},
		{[]string{"dock", "trackpad"}, true},
		{[]string{"input-sources"}, true},
	} {
		if got := NeedsLogout(tc.groups); got != tc.want {
			t.Errorf("NeedsLogout(%v) = %v, want %v", tc.groups, got, tc.want)
		}
	}
}
