// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"testing"

	"github.com/spf13/pflag"
)

// resetProposeFlags returns the propose command to its pre-parse state: the
// flag values and cobra's "changed" bookkeeping alike, since the command is a
// package-level singleton shared by every case.
func resetProposeFlags(t *testing.T) {
	t.Helper()
	gitProposeFlags.All = false
	gitProposeFlags.AutoMerge = autoMergeOff
	gitProposeFlags.Browse = false
	gitProposeFlags.Copy = false
	gitProposeFlags.Draft = false
	gitProposeCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
}

func TestProposeDraftExcludesAutoMerge(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"draft alone", []string{"--draft"}, false},
		{"auto-merge alone", []string{"--auto-merge=rebase"}, false},
		{"a draft cannot also be flagged to merge", []string{"--draft", "--auto-merge=rebase"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resetProposeFlags(t)
			t.Cleanup(func() { resetProposeFlags(t) })
			if err := gitProposeCmd.Flags().Parse(c.args); err != nil {
				t.Fatalf("parse %v: %v", c.args, err)
			}
			if err := gitProposeCmd.ValidateFlagGroups(); (err != nil) != c.wantErr {
				t.Fatalf("ValidateFlagGroups() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

// A configured auto-merge is a default, not an argument, so --draft may be
// given alongside it: the value lands but the flag stays unchanged, and
// propose drops it at run time rather than refusing the command line.
func TestProposeDraftToleratesConfiguredAutoMerge(t *testing.T) {
	resetProposeFlags(t)
	t.Cleanup(func() { resetProposeFlags(t) })
	if err := gitProposeCmd.Flags().Parse([]string{"--draft"}); err != nil {
		t.Fatalf("parse --draft: %v", err)
	}
	fake := &fakeGitConfig{values: map[string]string{"dotty.propose.auto-merge": "rebase"}}
	if err := applyGitConfigFlagDefaults(context.Background(), fake, gitProposeCmd); err != nil {
		t.Fatalf("applyGitConfigFlagDefaults: %v", err)
	}
	if gitProposeFlags.AutoMerge != "rebase" {
		t.Errorf("AutoMerge = %q, want %q from config", gitProposeFlags.AutoMerge, "rebase")
	}
	if err := gitProposeCmd.ValidateFlagGroups(); err != nil {
		t.Errorf("ValidateFlagGroups() error = %v, want nil for a config-sourced default", err)
	}
}

func TestAutoMergeModeSet(t *testing.T) {
	cases := []struct {
		in      string
		want    autoMergeMode
		wantErr bool
	}{
		{"merge", "merge", false},
		{"rebase", "rebase", false},
		{"squash", "squash", false},
		{"comment", autoMergeComment, false},
		{"REBASE", "rebase", false},
		{"COMMENT", autoMergeComment, false},
		{"true", "", true},
		{"false", "", true},
		{"yes", "", true},
		{"banana", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		t.Run("input "+c.in, func(t *testing.T) {
			var m autoMergeMode
			err := m.Set(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("Set(%q) error = %v, wantErr %v", c.in, err, c.wantErr)
			}
			if !c.wantErr && m != c.want {
				t.Errorf("Set(%q) = %q, want %q", c.in, m, c.want)
			}
		})
	}
}

func TestAutoMergeModeMergeMethod(t *testing.T) {
	cases := []struct {
		mode autoMergeMode
		want string
	}{
		{autoMergeOff, ""},
		{autoMergeComment, ""},
		{"merge", "merge"},
		{"rebase", "rebase"},
		{"squash", "squash"},
	}
	for _, c := range cases {
		if got := c.mode.mergeMethod(); got != c.want {
			t.Errorf("(%q).mergeMethod() = %q, want %q", c.mode, got, c.want)
		}
	}
}
