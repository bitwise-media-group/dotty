// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Worktree is one entry of `git worktree list`. Branch is the short branch
// name, empty for a detached HEAD or a bare main repository. Main marks the
// main working tree (always listed first); Prunable marks a registration
// whose directory is gone.
type Worktree struct {
	Path     string
	Branch   string
	Main     bool
	Locked   bool
	Prunable bool
}

// ListWorktrees lists every worktree registered with the repository at repo,
// main worktree first.
func ListWorktrees(ctx context.Context, r Runner, repo string) ([]Worktree, error) {
	out, err := r.Output(ctx, "git", "-C", repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	return parseWorktreeList(out), nil
}

// parseWorktreeList parses `git worktree list --porcelain`: blank-line
// separated records of "key [value]" lines, the first record being the main
// worktree.
func parseWorktreeList(out []byte) []Worktree {
	var list []Worktree
	var cur *Worktree
	for line := range strings.Lines(string(out)) {
		line = strings.TrimRight(line, "\r\n")
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			list = append(list, Worktree{Path: value, Main: len(list) == 0})
			cur = &list[len(list)-1]
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "locked":
			if cur != nil {
				cur.Locked = true
			}
		case "prunable":
			if cur != nil {
				cur.Prunable = true
			}
		case "":
			cur = nil
		}
	}
	return list
}

// PruneWorktrees drops the registrations of worktrees whose directories are
// gone, so their branches can be deleted.
func PruneWorktrees(ctx context.Context, r Runner, repo string) error {
	if _, err := r.Output(ctx, "git", "-C", repo, "worktree", "prune"); err != nil {
		return fmt.Errorf("prune worktrees: %w", err)
	}
	return nil
}

// WorktreeState measures the work a worktree would lose on removal.
// Uncommitted counts changed and untracked paths; Unpushed counts commits on
// HEAD that no remote-tracking ref and no other local branch contains —
// commits fast-forwarded into a local trunk are safe, not unpushed.
type WorktreeState struct {
	Uncommitted int
	Unpushed    int
}

// Dirty reports whether removing the worktree would lose work.
func (s WorktreeState) Dirty() bool {
	return s.Uncommitted > 0 || s.Unpushed > 0
}

// InspectWorktree measures the uncommitted and unpushed work in the worktree
// at path.
func InspectWorktree(ctx context.Context, r Runner, path string) (WorktreeState, error) {
	var s WorktreeState
	status, err := r.Output(ctx, "git", "-C", path, "status", "--porcelain")
	if err != nil {
		return s, fmt.Errorf("status of %s: %w", path, err)
	}
	for line := range strings.Lines(string(status)) {
		if strings.TrimSpace(line) != "" {
			s.Uncommitted++
		}
	}

	// --exclude patterns applied to --branches must omit the refs/heads/
	// prefix, hence --short.
	args := []string{"-C", path, "rev-list", "--count", "HEAD", "--not", "--remotes"}
	if out, err := r.Output(ctx, "git", "-C", path, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
		args = append(args, "--exclude="+strings.TrimSpace(string(out)))
	}
	args = append(args, "--branches")
	out, err := r.Output(ctx, "git", args...)
	if err != nil {
		return s, fmt.Errorf("count unpushed commits in %s: %w", path, err)
	}
	s.Unpushed, err = strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return s, fmt.Errorf("count unpushed commits in %s: %w", path, err)
	}
	return s, nil
}

// gitDirs resolves the absolute (symlink-free) git dir and common dir of the
// checkout at path. They differ exactly when path is a linked worktree.
func gitDirs(ctx context.Context, r Runner, path string) (gitDir, commonDir string, err error) {
	out, err := r.Output(ctx, "git", "-C", path, "rev-parse", "--path-format=absolute",
		"--git-dir", "--git-common-dir")
	if err != nil {
		return "", "", fmt.Errorf("resolve git dir of %s: %w", path, err)
	}
	gitDir, commonDir, ok := strings.Cut(strings.TrimSpace(string(out)), "\n")
	if !ok {
		return "", "", fmt.Errorf("resolve git dir of %s: unexpected output %q", path, out)
	}
	return realPath(gitDir), realPath(strings.TrimSpace(commonDir)), nil
}

// CommonDir returns the shared git directory of the checkout at path — equal
// across a repository's main and linked worktrees, so it identifies the
// repository.
func CommonDir(ctx context.Context, r Runner, path string) (string, error) {
	_, common, err := gitDirs(ctx, r, path)
	return common, err
}

// IsLinkedWorktree reports whether path is a linked worktree rather than a
// repository's main checkout.
func IsLinkedWorktree(ctx context.Context, r Runner, path string) (bool, error) {
	gitDir, common, err := gitDirs(ctx, r, path)
	if err != nil {
		return false, err
	}
	return gitDir != common, nil
}

// MainRepo resolves the repository that owns the checkout at path: the
// directory holding its common .git, or the common dir itself for a bare
// repository.
func MainRepo(ctx context.Context, r Runner, path string) (string, error) {
	common, err := CommonDir(ctx, r, path)
	if err != nil {
		return "", err
	}
	if filepath.Base(common) == ".git" {
		return filepath.Dir(common), nil
	}
	return common, nil
}

// RemoveWorktree removes the linked worktree at path from repo. A double
// --force removes dirty, locked, and submodule-carrying worktrees alike; if
// git still refuses, the directory is deleted outright and the stale
// registration unlocked and pruned. It fails only when the directory or its
// registration survives.
func RemoveWorktree(ctx context.Context, r Runner, repo, path string) error {
	_, removeErr := r.Output(ctx, "git", "-C", repo, "worktree", "remove", "--force", "--force", path)
	if removeErr == nil {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove worktree %s: %w", path, errors.Join(removeErr, err))
	}
	_, _ = r.Output(ctx, "git", "-C", repo, "worktree", "unlock", path)
	if err := PruneWorktrees(ctx, r, repo); err != nil {
		return fmt.Errorf("remove worktree %s: %w", path, errors.Join(removeErr, err))
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("remove worktree %s: %w", path, removeErr)
	}
	return nil
}

// realPath resolves symlinks in p (macOS's /var -> /private/var among them)
// so paths from different sources compare equal; p is returned unchanged
// when it cannot be resolved.
func realPath(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return filepath.Clean(p)
}
