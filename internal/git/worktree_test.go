// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"slices"
	"testing"
)

func TestParseWorktreeList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []Worktree
	}{
		{
			name: "empty output",
			in:   "",
			want: nil,
		},
		{
			name: "main only",
			in:   "worktree /r/repo\nHEAD abc\nbranch refs/heads/main\n\n",
			want: []Worktree{{Path: "/r/repo", Branch: "main", Main: true}},
		},
		{
			name: "linked branch with slash",
			in: "worktree /r/repo\nHEAD abc\nbranch refs/heads/main\n\n" +
				"worktree /w/repo-fix\nHEAD def\nbranch refs/heads/agent/repo-fix\n\n",
			want: []Worktree{
				{Path: "/r/repo", Branch: "main", Main: true},
				{Path: "/w/repo-fix", Branch: "agent/repo-fix"},
			},
		},
		{
			name: "detached",
			in: "worktree /r/repo\nHEAD abc\nbranch refs/heads/main\n\n" +
				"worktree /w/detached\nHEAD def\ndetached\n\n",
			want: []Worktree{
				{Path: "/r/repo", Branch: "main", Main: true},
				{Path: "/w/detached"},
			},
		},
		{
			name: "locked with and without reason",
			in: "worktree /r/repo\nHEAD abc\nbranch refs/heads/main\n\n" +
				"worktree /w/a\nHEAD def\nbranch refs/heads/a\nlocked\n\n" +
				"worktree /w/b\nHEAD 123\nbranch refs/heads/b\nlocked claude agent\n\n",
			want: []Worktree{
				{Path: "/r/repo", Branch: "main", Main: true},
				{Path: "/w/a", Branch: "a", Locked: true},
				{Path: "/w/b", Branch: "b", Locked: true},
			},
		},
		{
			name: "prunable",
			in: "worktree /r/repo\nHEAD abc\nbranch refs/heads/main\n\n" +
				"worktree /w/gone\nHEAD def\nbranch refs/heads/agent/gone\n" +
				"prunable gitdir file points to non-existent location\n\n",
			want: []Worktree{
				{Path: "/r/repo", Branch: "main", Main: true},
				{Path: "/w/gone", Branch: "agent/gone", Prunable: true},
			},
		},
		{
			name: "bare main",
			in:   "worktree /r/repo.git\nbare\n\nworktree /w/x\nHEAD def\nbranch refs/heads/x\n",
			want: []Worktree{
				{Path: "/r/repo.git", Main: true},
				{Path: "/w/x", Branch: "x"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseWorktreeList([]byte(c.in)); !slices.Equal(got, c.want) {
				t.Errorf("parseWorktreeList() = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestWorktreeStateDirty(t *testing.T) {
	cases := []struct {
		state WorktreeState
		want  bool
	}{
		{WorktreeState{}, false},
		{WorktreeState{Uncommitted: 1}, true},
		{WorktreeState{Unpushed: 2}, true},
	}
	for _, c := range cases {
		if got := c.state.Dirty(); got != c.want {
			t.Errorf("%+v.Dirty() = %v, want %v", c.state, got, c.want)
		}
	}
}
