// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Trunk is the configured integration branch remote and name
// (e.g. upstream + main, or origin + main when there is no upstream).
type Trunk struct {
	Remote string // "upstream" or "origin"
	Branch string // usually "main"
}

// Ref returns remote/branch for rev-parse and merge-base.
func (t Trunk) Ref() string {
	return t.Remote + "/" + t.Branch
}

// ResolveTrunk prefers upstream/main when the remote exists, else origin/main,
// else origin's HEAD default branch.
func ResolveTrunk(ctx context.Context, r Runner) (Trunk, error) {
	if hasRemote(ctx, r, "upstream") {
		return Trunk{Remote: "upstream", Branch: "main"}, nil
	}
	if hasRemote(ctx, r, "origin") {
		return Trunk{Remote: "origin", Branch: "main"}, nil
	}
	return Trunk{}, errors.New("no upstream or origin remote configured")
}

// PushRemote is where feature branches are pushed (the fork). Prefer origin.
func PushRemote(ctx context.Context, r Runner) (string, error) {
	if hasRemote(ctx, r, "origin") {
		return "origin", nil
	}
	return "", errors.New("no origin remote configured (push remote for stack branches)")
}

func hasRemote(ctx context.Context, r Runner, name string) bool {
	out, err := r.Output(ctx, "git", "remote")
	if err != nil {
		return false
	}
	return slices.Contains(strings.Fields(string(out)), name)
}

// RemoteURL returns the fetch URL for a remote.
func RemoteURL(ctx context.Context, r Runner, remote string) (string, error) {
	out, err := r.Output(ctx, "git", "remote", "get-url", remote)
	if err != nil {
		return "", fmt.Errorf("remote %s URL: %w", remote, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentBranch returns the short name of HEAD's branch.
func CurrentBranch(ctx context.Context, r Runner) (string, error) {
	out, err := r.Output(ctx, "git", "branch", "--show-current")
	if err != nil {
		return "", err
	}
	b := strings.TrimSpace(string(out))
	if b == "" {
		return "", errors.New("detached HEAD; check out a branch first")
	}
	return b, nil
}

// RevParse resolves a revision to a full SHA.
func RevParse(ctx context.Context, r Runner, rev string) (string, error) {
	out, err := r.Output(ctx, "git", "rev-parse", rev)
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", rev, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsAncestor reports whether anc is an ancestor of desc (inclusive).
func IsAncestor(ctx context.Context, r Runner, anc, desc string) (bool, error) {
	_, err := r.Output(ctx, "git", "merge-base", "--is-ancestor", anc, desc)
	if err == nil {
		return true, nil
	}
	// git exits 1 when not an ancestor; other errors are real failures.
	if exitCode(err) == 1 {
		return false, nil
	}
	return false, err
}

// Relation of a branch tip versus trunk.
type Relation string

const (
	RelMerged    Relation = "merged"    // tip is ancestor of trunk (already on main)
	RelIdentical Relation = "identical" // tip equals trunk
	RelFF        Relation = "ff"        // trunk is ancestor of tip (can FF-merge)
	RelDiverged  Relation = "diverged"  // neither is ancestor of the other
)

// ClassifyTip compares branch tip to trunk tip (after fetch, trunk is remote ref).
func ClassifyTip(ctx context.Context, r Runner, trunkRef, branch string) (Relation, error) {
	trunkSHA, err := RevParse(ctx, r, trunkRef)
	if err != nil {
		return "", fmt.Errorf("resolve trunk %s: %w", trunkRef, err)
	}
	tipSHA, err := RevParse(ctx, r, branch)
	if err != nil {
		return "", fmt.Errorf("resolve branch %s: %w", branch, err)
	}
	if trunkSHA == tipSHA {
		return RelIdentical, nil
	}
	tipOnTrunk, err := IsAncestor(ctx, r, tipSHA, trunkSHA)
	if err != nil {
		return "", err
	}
	if tipOnTrunk {
		return RelMerged, nil
	}
	trunkOnTip, err := IsAncestor(ctx, r, trunkSHA, tipSHA)
	if err != nil {
		return "", err
	}
	if trunkOnTip {
		return RelFF, nil
	}
	return RelDiverged, nil
}

// exitCode extracts a process exit code from a (possibly wrapped) value with
// an ExitCode method — *exec.ExitError in production — or -1 when there is
// none.
func exitCode(err error) int {
	var ec interface{ ExitCode() int }
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	return -1
}
