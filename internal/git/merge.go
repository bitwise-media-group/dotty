// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// CommitsNotIn returns how many commits are reachable from rev but not from
// contained — zero means contained already carries all of rev's history.
func CommitsNotIn(ctx context.Context, r Runner, rev, contained string) (int, error) {
	out, err := r.Output(ctx, "git", "rev-list", "--count", rev, "^"+contained)
	if err != nil {
		return 0, fmt.Errorf("count commits of %s not in %s: %w", rev, contained, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse rev-list count for %s: %w", rev, err)
	}
	return n, nil
}

// MergeParents collapses the n layers directly below branch into it: each
// absorbed parent's branch is deleted (local and remote, per cfg) and its
// layer removed from the stack metadata. branch's history does not change —
// a stacked child already contains its parents' commits, which every parent
// is verified for first, so the merge is pure bookkeeping. It returns the
// updated stack and the absorbed branch names, bottom-most first.
func MergeParents(ctx context.Context, r Runner, s Stack, branch string, n int,
	cfg CleanupConfig) (Stack, []string, error) {
	i := s.IndexOf(branch)
	if i < 0 {
		return s, nil, fmt.Errorf("branch %q is %w", branch, ErrNotInStack)
	}
	if n < 1 {
		return s, nil, errors.New("nothing to merge: need at least one parent layer")
	}
	if n > i {
		return s, nil, fmt.Errorf("%s has %d parent layer(s) in the stack, not %d", branch, i, n)
	}

	parents := make([]string, 0, n)
	for _, l := range s.Layers[i-n : i] {
		parents = append(parents, l.Branch)
	}
	for _, parent := range parents {
		missing, err := CommitsNotIn(ctx, r, parent, branch)
		if err != nil {
			return s, nil, err
		}
		if missing > 0 {
			return s, nil, fmt.Errorf(
				"layer %s has %d commit(s) missing from %s; the stack below is not in sync",
				parent, missing, branch)
		}
	}

	var err error
	for _, parent := range parents {
		if cfg.Enabled {
			_ = DeleteBranchLocal(ctx, r, parent)
			if remote, rerr := PushRemote(ctx, r); rerr == nil {
				_ = DeleteBranchRemote(ctx, r, remote, parent)
			}
		}
		if s, err = RemoveLayer(ctx, r, s, parent); err != nil {
			return s, nil, err
		}
	}
	return s, parents, nil
}
