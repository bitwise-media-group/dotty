// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitEnv isolates git from the user's configuration and gives commits an
// identity, pointing HOME and the XDG dirs at scratch space (no active
// profile) and DOTTY_WORKTREES at a shared root under it. It returns the
// scratch home.
func gitEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@x")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@x")
	t.Setenv("DOTTY_WORKTREES", filepath.Join(home, "worktrees"))
	t.Setenv("TMUX", "")
	return home
}

// gitT runs git in dir, failing the test on error, and returns its trimmed
// stdout. file:// submodules are allowed for the submodule fixtures.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-c", "protocol.file.allow=always", "-c", "commit.gpgsign=false", "-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo creates a repository at dir with one commit on main.
func newRepo(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "init", "-b", "main")
	gitT(t, dir, "commit", "--allow-empty", "-m", "init")
	return dir
}

// worktreeRepo builds a real repository under a scratch home and returns it.
func worktreeRepo(t *testing.T) string {
	t.Helper()
	home := gitEnv(t)
	return newRepo(t, filepath.Join(home, "Repos", "proj"))
}

// worktreePath is where start puts suffix's worktree for the proj repository
// under the shared root gitEnv configures.
func worktreePath(suffix string) string {
	return filepath.Join(os.Getenv("DOTTY_WORKTREES"), "proj-"+suffix)
}

// execWorktree runs `dotty git worktree <args>`, resetting remove's flags
// afterwards — cobra keeps flag values between executions.
func execWorktree(t *testing.T, args ...string) error {
	t.Helper()
	t.Cleanup(func() {
		gitWorktreeRemoveFlags.Yes = false
		for _, name := range []string{"yes", "force"} {
			gitWorktreeRemoveCmd.Flags().Lookup(name).Changed = false
		}
	})
	return execDotty(t, append([]string{"git", "worktree"}, args...)...)
}

func branchExists(t *testing.T, repo, branch string) bool {
	t.Helper()
	return exec.Command("git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run() == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func TestGitWorktreeAddAndRemove(t *testing.T) {
	repo := worktreeRepo(t)
	want := worktreePath("fix-1")

	if err := execWorktree(t, "add", repo, "fix-1"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !dirExists(want) {
		t.Fatalf("worktree not created at %s", want)
	}
	if got := gitT(t, want, "rev-parse", "--abbrev-ref", "HEAD"); got != "agent/proj-fix-1" {
		t.Fatalf("worktree branch = %q, want agent/proj-fix-1", got)
	}

	// add is idempotent: an existing worktree of this repo is reused. start
	// survives as an alias of add.
	if err := execWorktree(t, "start", repo, "fix-1"); err != nil {
		t.Fatalf("re-add via start: %v", err)
	}

	// end survives as an alias of remove.
	if err := execWorktree(t, "end", want); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if dirExists(want) {
		t.Fatal("worktree still present after remove")
	}
	if branchExists(t, repo, "agent/proj-fix-1") {
		t.Fatal("agent branch survived remove")
	}
	// Removing an already-gone worktree is quiet, not an error.
	if err := execWorktree(t, "remove", want); err != nil {
		t.Fatalf("re-remove: %v", err)
	}
}

func TestGitWorktreeRemoveSparesNonAgentBranches(t *testing.T) {
	repo := worktreeRepo(t)
	path := filepath.Join(t.TempDir(), "manual")
	gitT(t, repo, "worktree", "add", "-b", "feature/manual", path)

	if err := execWorktree(t, "remove", path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if dirExists(path) {
		t.Fatal("worktree still present")
	}
	if !branchExists(t, repo, "feature/manual") {
		t.Fatal("non-agent branch was deleted")
	}
}

func TestGitWorktreeAddRoots(t *testing.T) {
	cases := []struct {
		name    string
		setting func(home, repo string) string
		want    func(home, repo string) string
	}{
		{
			name:    "absolute root",
			setting: func(home, _ string) string { return filepath.Join(home, "wt-root") },
			want:    func(home, _ string) string { return filepath.Join(home, "wt-root", "proj-x") },
		},
		{
			name:    "repo-relative legacy root",
			setting: func(_, _ string) string { return ".worktrees" },
			want:    func(_, repo string) string { return filepath.Join(repo, ".worktrees", "proj-x") },
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := worktreeRepo(t)
			home := os.Getenv("HOME")
			t.Setenv("DOTTY_WORKTREES", c.setting(home, repo))

			if err := execWorktree(t, "add", repo, "x"); err != nil {
				t.Fatalf("add: %v", err)
			}
			if want := c.want(home, repo); !dirExists(want) {
				t.Fatalf("worktree not at %s", want)
			}
		})
	}
}

// TestGitWorktreeAddRefusesForeignRepo pins the shared-root collision: two
// repositories with the same basename derive the same worktree name, and the
// second must not be handed the first one's checkout.
func TestGitWorktreeAddRefusesForeignRepo(t *testing.T) {
	home := gitEnv(t)
	first := newRepo(t, filepath.Join(home, "Repos", "a", "proj"))
	second := newRepo(t, filepath.Join(home, "Repos", "b", "proj"))

	if err := execWorktree(t, "add", first, "fix"); err != nil {
		t.Fatalf("start first: %v", err)
	}
	err := execWorktree(t, "add", second, "fix")
	if err == nil || !strings.Contains(err.Error(), "not a worktree of") {
		t.Fatalf("start second = %v, want a not-a-worktree-of error", err)
	}
	if branchExists(t, second, "agent/proj-fix") {
		t.Error("second repository gained an agent branch")
	}
}

func TestGitWorktreeRemoveRefusesMainCheckout(t *testing.T) {
	repo := worktreeRepo(t)
	err := execWorktree(t, "remove", "--yes", repo)
	if err == nil || !strings.Contains(err.Error(), "main checkout") {
		t.Fatalf("remove main = %v, want a main-checkout refusal", err)
	}
	if !dirExists(filepath.Join(repo, ".git")) {
		t.Fatal("main checkout was damaged")
	}
}

func TestGitWorktreeRemoveLostWork(t *testing.T) {
	cases := []struct {
		name  string
		dirty func(t *testing.T, path string)
	}{
		{
			name: "uncommitted file",
			dirty: func(t *testing.T, path string) {
				if err := os.WriteFile(filepath.Join(path, "scratch.txt"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unpushed commit",
			dirty: func(t *testing.T, path string) {
				gitT(t, path, "commit", "--allow-empty", "-m", "agent work")
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := worktreeRepo(t)
			path := worktreePath("dirty")
			if err := execWorktree(t, "add", repo, "dirty"); err != nil {
				t.Fatalf("add: %v", err)
			}
			c.dirty(t, path)

			// Non-interactive without --yes: refused, everything kept.
			err := execWorktree(t, "remove", path)
			if err == nil || !strings.Contains(err.Error(), "--yes") {
				t.Fatalf("remove without --yes = %v, want a pass --yes error", err)
			}
			if !dirExists(path) || !branchExists(t, repo, "agent/proj-dirty") {
				t.Fatal("refused removal still removed the worktree or branch")
			}

			if err := execWorktree(t, "remove", "-f", path); err != nil {
				t.Fatalf("remove -f: %v", err)
			}
			if dirExists(path) || branchExists(t, repo, "agent/proj-dirty") {
				t.Fatal("worktree or branch survived remove -f")
			}
		})
	}
}

// TestGitWorktreeRemoveCommitFastForwardedLocally pins the lost-work measure:
// a commit already on a local trunk is safe even with no remote at all.
func TestGitWorktreeRemoveCommitFastForwardedLocally(t *testing.T) {
	repo := worktreeRepo(t)
	path := worktreePath("ff")
	if err := execWorktree(t, "add", repo, "ff"); err != nil {
		t.Fatalf("add: %v", err)
	}
	gitT(t, path, "commit", "--allow-empty", "-m", "agent work")
	gitT(t, repo, "merge", "--ff-only", "agent/proj-ff")

	if err := execWorktree(t, "remove", path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if dirExists(path) {
		t.Fatal("worktree survived remove")
	}
}

func TestGitWorktreeRemoveLocked(t *testing.T) {
	repo := worktreeRepo(t)
	path := worktreePath("locked")
	if err := execWorktree(t, "add", repo, "locked"); err != nil {
		t.Fatalf("add: %v", err)
	}
	gitT(t, repo, "worktree", "lock", "--reason", "agent running", path)

	if err := execWorktree(t, "remove", path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if dirExists(path) {
		t.Fatal("locked worktree survived remove")
	}
	if list := gitT(t, repo, "worktree", "list", "--porcelain"); strings.Contains(list, path) {
		t.Errorf("worktree still registered:\n%s", list)
	}
}

// TestGitWorktreeRemoveSubmodule pins removal of a worktree with a populated
// submodule — plain `git worktree remove` refuses those.
func TestGitWorktreeRemoveSubmodule(t *testing.T) {
	repo := worktreeRepo(t)
	sub := newRepo(t, filepath.Join(os.Getenv("HOME"), "Repos", "sub"))
	gitT(t, repo, "submodule", "add", sub, "sub")
	gitT(t, repo, "commit", "-m", "add submodule")

	path := worktreePath("sub")
	if err := execWorktree(t, "add", repo, "sub"); err != nil {
		t.Fatalf("add: %v", err)
	}
	gitT(t, path, "submodule", "update", "--init")
	if !dirExists(filepath.Join(path, "sub", ".git")) && !fileExists(filepath.Join(path, "sub", ".git")) {
		t.Fatal("submodule not populated")
	}

	if err := execWorktree(t, "remove", "--yes", path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if dirExists(path) {
		t.Fatal("worktree with submodule survived remove")
	}
	if branchExists(t, repo, "agent/proj-sub") {
		t.Fatal("agent branch survived remove")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
