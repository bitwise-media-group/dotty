// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// realGitRunner shells out to a real git binary in a temp repo.
type realGitRunner struct {
	dir string
}

func (r *realGitRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.dir
	return cmd.Output()
}

func (r *realGitRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *realGitRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	return r.Run(ctx, name, args...)
}

// gitRun runs git in dir with a fixed test identity, failing the test on any
// error.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initRepo(t *testing.T) (*realGitRunner, context.Context) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	ctx := context.Background()
	r := &realGitRunner{dir: dir}
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")
	// Global/user git config may require SSH signing; disable in the fixture.
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	gitRun(t, dir, "config", "tag.gpgsign", "false")
	// Fake origin so ResolveTrunk works (points at self).
	gitRun(t, dir, "remote", "add", "origin", dir)
	// Initial commit on main.
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "README")
	gitRun(t, dir, "commit", "-m", "chore: root")
	// origin/main tracking: fetch into itself
	gitRun(t, dir, "update-ref", "refs/remotes/origin/main", "main")
	return r, ctx
}

func commitFile(t *testing.T, dir, name, body, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", name)
	gitRun(t, dir, "commit", "-m", msg)
}

func TestDiscoverStackThreeNodes(t *testing.T) {
	r, ctx := initRepo(t)
	// main <- a <- b  (3 nodes with trunk)
	gitRun(t, r.dir, "checkout", "-b", "feat-a")
	commitFile(t, r.dir, "a.txt", "a\n", "feat: a")
	gitRun(t, r.dir, "checkout", "-b", "feat-b")
	commitFile(t, r.dir, "b.txt", "b\n", "feat: b")
	gitRun(t, r.dir, "update-ref", "refs/remotes/origin/main", "main")

	trunk := Trunk{Remote: "origin", Branch: "main"}
	s, ok, err := DiscoverStack(ctx, r, trunk, "feat-b")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected discovery")
	}
	if len(s.Layers) != 2 || s.Layers[0].Branch != "feat-a" || s.Layers[1].Branch != "feat-b" {
		t.Fatalf("layers = %+v", s.Layers)
	}
}

func TestDiscoverStackTwoNodesNotEnough(t *testing.T) {
	r, ctx := initRepo(t)
	gitRun(t, r.dir, "checkout", "-b", "solo")
	commitFile(t, r.dir, "s.txt", "s\n", "feat: solo")
	gitRun(t, r.dir, "update-ref", "refs/remotes/origin/main", "main")

	trunk := Trunk{Remote: "origin", Branch: "main"}
	_, ok, err := DiscoverStack(ctx, r, trunk, "solo")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("single feature branch must not count as a stack")
	}
}

func TestDiscoverStackExtendsToTip(t *testing.T) {
	r, ctx := initRepo(t)
	gitRun(t, r.dir, "checkout", "-b", "l1")
	commitFile(t, r.dir, "1.txt", "1\n", "feat: 1")
	gitRun(t, r.dir, "checkout", "-b", "l2")
	commitFile(t, r.dir, "2.txt", "2\n", "feat: 2")
	gitRun(t, r.dir, "checkout", "-b", "l3")
	commitFile(t, r.dir, "3.txt", "3\n", "feat: 3")
	gitRun(t, r.dir, "checkout", "l2")
	gitRun(t, r.dir, "update-ref", "refs/remotes/origin/main", "main")

	trunk := Trunk{Remote: "origin", Branch: "main"}
	// On l2, linear extension should still include l3.
	s, ok, err := DiscoverStack(ctx, r, trunk, "l2")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected discovery")
	}
	if len(s.Layers) != 3 {
		t.Fatalf("want 3 layers, got %+v", s.Layers)
	}
	if s.Layers[0].Branch != "l1" || s.Layers[2].Branch != "l3" {
		t.Fatalf("layers = %+v", s.Layers)
	}
}

func TestLoadOrDiscoverPersists(t *testing.T) {
	r, ctx := initRepo(t)
	gitRun(t, r.dir, "checkout", "-b", "a")
	commitFile(t, r.dir, "a.txt", "a\n", "feat: a")
	gitRun(t, r.dir, "checkout", "-b", "b")
	commitFile(t, r.dir, "b.txt", "b\n", "feat: b")
	gitRun(t, r.dir, "update-ref", "refs/remotes/origin/main", "main")

	trunk := Trunk{Remote: "origin", Branch: "main"}
	s1, discovered, err := LoadOrDiscoverStack(ctx, r, trunk)
	if err != nil || !discovered {
		t.Fatalf("discover: %v discovered=%v", err, discovered)
	}
	s2, discovered2, err := LoadOrDiscoverStack(ctx, r, trunk)
	if err != nil || discovered2 {
		t.Fatalf("second load should use config: %v discovered=%v", err, discovered2)
	}
	if s1.ID != s2.ID || len(s2.Layers) != 2 {
		t.Fatalf("s1=%+v s2=%+v", s1, s2)
	}
}
