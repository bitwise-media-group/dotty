// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// doneRepo builds a bare origin and a clone of it with main pushed, chdirs
// into the clone, and returns the clone.
func doneRepo(t *testing.T) string {
	t.Helper()
	home := gitEnv(t)
	seed := newRepo(t, filepath.Join(home, "seed"))
	origin := filepath.Join(home, "origin.git")
	gitT(t, home, "clone", "--bare", seed, origin)
	clone := filepath.Join(home, "Repos", "proj")
	gitT(t, home, "clone", origin, clone)
	t.Chdir(clone)
	return clone
}

// mergedAgentWorktree starts an agent worktree, commits in it, and
// fast-forwards main onto that commit without pushing — so the branch is
// merged into main but nowhere on the remote, and git done's own
// fast-forward + push publish it.
func mergedAgentWorktree(t *testing.T, repo, suffix string) string {
	t.Helper()
	if err := execWorktree(t, "add", repo, suffix); err != nil {
		t.Fatalf("add: %v", err)
	}
	path := worktreePath(suffix)
	gitT(t, path, "commit", "--allow-empty", "-m", "agent "+suffix)
	gitT(t, repo, "push", "origin", "agent/proj-"+suffix+":main")
	return path
}

func TestGitDoneRemovesMergedWorktrees(t *testing.T) {
	repo := doneRepo(t)
	path := mergedAgentWorktree(t, repo, "landed")

	if err := execDotty(t, "git", "done"); err != nil {
		t.Fatalf("git done: %v", err)
	}
	if dirExists(path) {
		t.Fatal("merged worktree survived git done")
	}
	if branchExists(t, repo, "agent/proj-landed") {
		t.Fatal("merged agent branch survived git done")
	}
}

func TestGitDoneKeepsDirtyMergedWorktree(t *testing.T) {
	repo := doneRepo(t)
	path := mergedAgentWorktree(t, repo, "dirty")
	if err := os.WriteFile(filepath.Join(path, "wip.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := execDotty(t, "git", "done"); err != nil {
		t.Fatalf("git done: %v", err)
	}
	if !dirExists(path) {
		t.Fatal("dirty merged worktree removed without confirmation")
	}
	if !branchExists(t, repo, "agent/proj-dirty") {
		t.Fatal("branch of a kept worktree was deleted")
	}
}
