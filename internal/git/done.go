// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// FetchAllPrune fetches every remote and prunes deleted tracking refs.
func FetchAllPrune(ctx context.Context, r Runner) error {
	return r.Run(ctx, "git", "fetch", "--all", "--prune")
}

// MergedLocalBranches lists local branches whose tips are already contained in
// trunk, excluding the trunk branch itself.
func MergedLocalBranches(ctx context.Context, r Runner, trunk Trunk) ([]string, error) {
	out, err := r.Output(ctx, "git", "branch", "--merged", trunk.Ref(),
		"--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("list merged branches: %w", err)
	}
	var merged []string
	for line := range strings.Lines(string(out)) {
		b := strings.TrimSpace(line)
		if b == "" || b == trunk.Branch {
			continue
		}
		merged = append(merged, b)
	}
	return merged, nil
}

// MergedRemoteBranches lists remote's branches whose tips are already
// contained in trunk, excluding the trunk branch and the symbolic HEAD.
// Names are returned without the remote/ prefix, ready for push --delete.
func MergedRemoteBranches(ctx context.Context, r Runner, remote string, trunk Trunk) ([]string, error) {
	out, err := r.Output(ctx, "git", "branch", "-r", "--merged", trunk.Ref(),
		"--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("list merged %s branches: %w", remote, err)
	}
	prefix := remote + "/"
	var merged []string
	for line := range strings.Lines(string(out)) {
		name, ok := strings.CutPrefix(strings.TrimSpace(line), prefix)
		if !ok || name == "" || name == "HEAD" || name == trunk.Branch {
			continue
		}
		merged = append(merged, name)
	}
	return merged, nil
}

// FastForward fast-forward-merges ref into the current branch; a diverged
// branch is an error, never a merge commit.
func FastForward(ctx context.Context, r Runner, ref string) error {
	if err := r.Run(ctx, "git", "merge", "--ff-only", ref); err != nil {
		return fmt.Errorf("fast-forward to %s: %w", ref, err)
	}
	return nil
}

// PushTrunk pushes the local trunk branch to remote. After a fast-forward
// this brings origin in step with the trunk remote — in fork workflows
// origin/main lags upstream/main until pushed; with a single remote it is a
// no-op.
func PushTrunk(ctx context.Context, r Runner, remote string, trunk Trunk) error {
	if err := r.Run(ctx, "git", "push", remote, trunk.Branch); err != nil {
		return fmt.Errorf("push %s to %s: %w", trunk.Branch, remote, err)
	}
	return nil
}

// ListStacks loads every stack recorded in local git config, ordered by ID.
func ListStacks(ctx context.Context, r Runner) ([]Stack, error) {
	out, err := r.Output(ctx, "git", "config", "--local",
		"--get-regexp", `^dotty\.stack\..+\.layers$`)
	if err != nil {
		// git config --get-regexp exits 1 when nothing matches.
		if exitCode(err) == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("list stacks: %w", err)
	}
	var ids []string
	for line := range strings.Lines(string(out)) {
		key, _, _ := strings.Cut(strings.TrimSpace(line), " ")
		id := strings.TrimSuffix(strings.TrimPrefix(key, "dotty.stack."), ".layers")
		if id != "" && id != key {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	stacks := make([]Stack, 0, len(ids))
	for _, id := range ids {
		s, err := loadStackByID(ctx, r, id)
		if err != nil {
			return nil, err
		}
		stacks = append(stacks, s)
	}
	return stacks, nil
}
