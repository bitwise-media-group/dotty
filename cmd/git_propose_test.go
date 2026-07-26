// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/bitwise-media-group/dotty/internal/git"
)

// fakeLayerLog answers the one `git log` LayerCommits issues, joining commits
// with the NUL/RS framing that format string produces.
type fakeLayerLog struct {
	commits []git.Commit
	err     error
	spec    string // the rev range the log was asked for
}

func (f *fakeLayerLog) Output(_ context.Context, _ string, args ...string) ([]byte, error) {
	f.spec = args[len(args)-1]
	if f.err != nil {
		return nil, f.err
	}
	var b strings.Builder
	for _, c := range f.commits {
		b.WriteString(c.SHA + "\x00" + c.Subject + "\x00" + c.Body + "\x1e")
	}
	return []byte(b.String()), nil
}

func (f *fakeLayerLog) Run(context.Context, string, ...string) error            { return nil }
func (f *fakeLayerLog) RunInteractive(context.Context, string, ...string) error { return nil }

func TestLayerTitleCommit(t *testing.T) {
	one := git.Commit{SHA: "aaaaaaaaaaaa", Subject: "feat: only", Body: "sole body"}
	two := git.Commit{SHA: "bbbbbbbbbbbb", Subject: "feat: second", Body: "second body"}
	cases := []struct {
		name     string
		commits  []git.Commit
		logErr   error
		titleSHA string
		want     string // wanted subject; empty means no commit resolves
		wantSpec string
	}{
		{
			name:     "a single-commit layer needs no nomination",
			commits:  []git.Commit{one},
			want:     "feat: only",
			wantSpec: "feat-a..feat-b",
		},
		{
			name:     "an abbreviated nomination still resolves",
			commits:  []git.Commit{one, two},
			titleSHA: "bbbbbbb",
			want:     "feat: second",
		},
		{
			name:    "a multi-commit layer with no nomination does not resolve",
			commits: []git.Commit{one, two},
		},
		{
			name:     "a nomination the rebase dropped does not resolve",
			commits:  []git.Commit{one, two},
			titleSHA: "cccccccc",
		},
		{
			name:   "an unreadable log does not resolve",
			logErr: errors.New("fatal: bad revision"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := git.Stack{ID: "s1", Layers: []git.Layer{
				{Branch: "feat-a"},
				{Branch: "feat-b", TitleSHA: c.titleSHA},
			}}
			fake := &fakeLayerLog{commits: c.commits, err: c.logErr}
			got, ok := layerTitleCommit(context.Background(), fake, s, 1,
				git.Trunk{Remote: "upstream", Branch: "main"})
			if ok != (c.want != "") {
				t.Fatalf("layerTitleCommit() ok = %v, want %v", ok, c.want != "")
			}
			if ok && got.Subject != c.want {
				t.Errorf("layerTitleCommit() subject = %q, want %q", got.Subject, c.want)
			}
			// The exclusive lower bound is the layer below, not trunk.
			if c.wantSpec != "" && fake.spec != c.wantSpec {
				t.Errorf("logged %q, want %q", fake.spec, c.wantSpec)
			}
		})
	}
}

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
