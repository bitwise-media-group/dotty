// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RebaseState is persisted under .git/dotty/stack-rebase.json for --continue/--abort.
type RebaseState struct {
	StackID      string   `json:"stack_id"`
	TrunkRemote  string   `json:"trunk_remote"`
	TrunkBranch  string   `json:"trunk_branch"`
	OpenBranches []string `json:"open_branches"`         // bottom → tip remaining
	Index        int      `json:"index"`                 // which open branch is mid-rebase
	Phase        string   `json:"phase"`                 // "rebase" | "resign"
	OrigBranch   string   `json:"orig_branch,omitempty"` // branch to restore when the sync completes
	Rewritten    []string `json:"rewritten,omitempty"`   // branches whose tips changed (need force-push)
}

const rebaseStateRel = "dotty/stack-rebase.json"

// GitDir returns the absolute path to .git (or the common dir).
func GitDir(ctx context.Context, r Runner) (string, error) {
	out, err := r.Output(ctx, "git", "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		root, err := r.Output(ctx, "git", "rev-parse", "--show-toplevel")
		if err != nil {
			return "", err
		}
		dir = filepath.Join(strings.TrimSpace(string(root)), dir)
	}
	return dir, nil
}

func rebaseStatePath(gitDir string) string {
	return filepath.Join(gitDir, rebaseStateRel)
}

// WriteRebaseState saves in-progress sync rebase state.
func WriteRebaseState(ctx context.Context, r Runner, st RebaseState) error {
	gd, err := GitDir(ctx, r)
	if err != nil {
		return err
	}
	path := rebaseStatePath(gd)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// ReadRebaseState loads state, or nil if none.
func ReadRebaseState(ctx context.Context, r Runner) (*RebaseState, error) {
	gd, err := GitDir(ctx, r)
	if err != nil {
		return nil, err
	}
	path := rebaseStatePath(gd)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var st RebaseState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// ClearRebaseState removes the state file.
func ClearRebaseState(ctx context.Context, r Runner) error {
	gd, err := GitDir(ctx, r)
	if err != nil {
		return err
	}
	_ = os.Remove(rebaseStatePath(gd))
	return nil
}

// AbortSyncRebase aborts a git rebase, returns HEAD to the branch the sync
// started on, and clears state.
func AbortSyncRebase(ctx context.Context, r Runner) error {
	st, _ := ReadRebaseState(ctx, r)
	_ = r.Run(ctx, "git", "rebase", "--abort")
	if st != nil && st.OrigBranch != "" {
		if err := Checkout(ctx, r, st.OrigBranch); err != nil {
			return err
		}
	}
	return ClearRebaseState(ctx, r)
}

// RebaseOnto rebases branch onto newbase (checkout branch first).
func RebaseOnto(ctx context.Context, r Runner, branch, newbase string) error {
	if err := Checkout(ctx, r, branch); err != nil {
		return err
	}
	if err := r.RunInteractive(ctx, "git", "rebase", newbase); err != nil {
		return fmt.Errorf("rebase %s onto %s: %w", branch, newbase, err)
	}
	return nil
}

// ForcePushBranch force-with-lease pushes branch to origin.
func ForcePushBranch(ctx context.Context, r Runner, branch string) error {
	remote, err := PushRemote(ctx, r)
	if err != nil {
		return err
	}
	return r.Run(ctx, "git", "push", "--force-with-lease", remote, branch)
}

// PRContent is the title and body a pull request carries. A stacked layer
// owns both: they are rebuilt from the commit the layer nominated, never read
// back from GitHub as authoritative.
type PRContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// UpdatePR rewrites an existing PR's title and body via gh.
func UpdatePR(ctx context.Context, r Runner, pr int, want PRContent, baseRemote string) error {
	repo, err := ghRepoFromRemote(ctx, r, baseRemote)
	if err != nil {
		return err
	}
	return r.Run(ctx, "gh", "pr", "edit", strconv.Itoa(pr),
		"--repo", repo, "--title", want.Title, "--body", want.Body)
}

// PRText fetches a PR's current title and body via gh, so callers can skip the
// edit when neither would change.
func PRText(ctx context.Context, r Runner, pr int, baseRemote string) (PRContent, error) {
	repo, err := ghRepoFromRemote(ctx, r, baseRemote)
	if err != nil {
		return PRContent{}, err
	}
	out, err := r.Output(ctx, "gh", "pr", "view", strconv.Itoa(pr),
		"--repo", repo, "--json", "title,body")
	if err != nil {
		return PRContent{}, fmt.Errorf("gh pr view #%d: %w", pr, err)
	}
	var current PRContent
	if err := json.Unmarshal(out, &current); err != nil {
		return PRContent{}, fmt.Errorf("parse gh pr view #%d: %w", pr, err)
	}
	return current, nil
}
