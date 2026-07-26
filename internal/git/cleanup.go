// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"strings"
)

// CleanupConfig controls branch deletion after merge detection.
type CleanupConfig struct {
	// Enabled deletes local and origin branches when a layer is merged.
	// Default true (matches git done-style cleanup).
	Enabled bool
}

// DefaultCleanup returns the default cleanup policy. `git config
// dotty.stack.cleanup false` disables it — read from the effective config so
// a global setting counts, not just a repo-local one.
func DefaultCleanup(ctx context.Context, r Runner) CleanupConfig {
	v, found, err := ConfigLookup(ctx, r, "dotty.stack.cleanup")
	if err == nil && found && (v == "false" || v == "0" || v == "no") {
		return CleanupConfig{Enabled: false}
	}
	return CleanupConfig{Enabled: true}
}

// DeleteBranchLocal removes a local branch (force if needed for merged tips).
func DeleteBranchLocal(ctx context.Context, r Runner, branch string) error {
	// Prefer -d; fall back to -D if git refuses (already merged should accept -d).
	if err := r.Run(ctx, "git", "branch", "-d", branch); err != nil {
		if err2 := r.Run(ctx, "git", "branch", "-D", branch); err2 != nil {
			return fmt.Errorf("delete local branch %s: %w", branch, err)
		}
	}
	return nil
}

// DeleteBranchRemote deletes origin/branch if it exists.
func DeleteBranchRemote(ctx context.Context, r Runner, remote, branch string) error {
	// Check if remote tracking ref exists.
	if _, err := RevParse(ctx, r, remote+"/"+branch); err != nil {
		return nil // nothing to delete
	}
	if err := r.Run(ctx, "git", "push", remote, "--delete", branch); err != nil {
		return fmt.Errorf("delete %s/%s: %w", remote, branch, err)
	}
	return nil
}

// LandedLayers returns the branches of every layer whose work is already on
// trunk, bottom-most first — the layers to drop before proposing or rebasing
// what is left.
//
// A tip strictly behind trunk (RelMerged) has landed by definition. A tip
// equal to trunk (RelIdentical) is ambiguous in local history alone: either
// the layer landed by fast-forward, moving trunk up onto it, or it is an empty
// layer freshly cut from trunk that never gained a commit. Only the remote
// separates the two, so an identical layer counts as landed exactly when it
// carries a pull request GitHub reports as merged. A layer with no PR — and
// one whose PR cannot be queried, offline or without gh — stays put.
func LandedLayers(ctx context.Context, r Runner, rows []LayerStatus, baseRemote string) []string {
	var landed []string
	for _, row := range rows {
		switch row.Relation {
		case RelMerged:
			landed = append(landed, row.Branch)
		case RelIdentical:
			if row.PR == 0 {
				continue
			}
			if merged, err := PRMerged(ctx, r, baseRemote, row.PR); err == nil && merged {
				landed = append(landed, row.Branch)
			}
		}
	}
	return landed
}

// CleanupMergedLayer deletes local+origin branch and removes lineage entry.
func CleanupMergedLayer(ctx context.Context, r Runner, s Stack, branch string, cfg CleanupConfig) (Stack, error) {
	cur, _ := CurrentBranch(ctx, r)
	if cur == branch {
		// Move off the branch before deleting: prefer remaining tip or trunk.
		var dest string
		for _, l := range s.Layers {
			if l.Branch != branch {
				dest = l.Branch
			}
		}
		if dest == "" {
			if trunk, err := ResolveTrunk(ctx, r); err == nil {
				_ = r.Run(ctx, "git", "checkout", trunk.Ref())
			}
		} else {
			_ = Checkout(ctx, r, dest)
		}
	}
	if cfg.Enabled {
		_ = DeleteBranchLocal(ctx, r, branch)
		if remote, err := PushRemote(ctx, r); err == nil {
			_ = DeleteBranchRemote(ctx, r, remote, branch)
		}
	}
	return RemoveLayer(ctx, r, s, branch)
}

// FetchTrunk updates the trunk remote ref.
func FetchTrunk(ctx context.Context, r Runner, trunk Trunk) error {
	return r.Run(ctx, "git", "fetch", trunk.Remote, trunk.Branch)
}

// FetchPushRemote updates origin.
func FetchPushRemote(ctx context.Context, r Runner) error {
	remote, err := PushRemote(ctx, r)
	if err != nil {
		return err
	}
	return r.Run(ctx, "git", "fetch", remote)
}

// CommitSubject returns the subject of a commit.
func CommitSubject(ctx context.Context, r Runner, rev string) (string, error) {
	out, err := r.Output(ctx, "git", "log", "-1", "--format=%s", rev)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
