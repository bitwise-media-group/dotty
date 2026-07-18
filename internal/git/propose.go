// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ResolveProposeScope computes the inclusive top layer index to propose
// (0-based): the tip with all, otherwise the current branch's layer.
func ResolveProposeScope(s Stack, currentBranch string, all bool) (through int, err error) {
	if all {
		return len(s.Layers) - 1, nil
	}
	i := s.IndexOf(currentBranch)
	if i < 0 {
		return 0, fmt.Errorf("current branch %q is %w", currentBranch, ErrNotInStack)
	}
	return i, nil
}

// PushBranch pushes branch to the push remote with -u.
func PushBranch(ctx context.Context, r Runner, branch string) error {
	remote, err := PushRemote(ctx, r)
	if err != nil {
		return err
	}
	return r.Run(ctx, "git", "push", "-u", remote, branch)
}

// PRTarget resolves the PR base (trunk remote and branch), verifying a push
// remote for the head branches exists.
func PRTarget(ctx context.Context, r Runner) (baseRemote, baseBranch string, err error) {
	trunk, err := ResolveTrunk(ctx, r)
	if err != nil {
		return "", "", err
	}
	if _, err := PushRemote(ctx, r); err != nil {
		return "", "", err
	}
	return trunk.Remote, trunk.Branch, nil
}

// CreateOrUpdatePR opens or updates a PR for branch against trunk using gh.
// title and body are the PR content. Returns the PR number.
func CreateOrUpdatePR(ctx context.Context, r Runner, branch string, existingPR int,
	title, body, baseRemote, baseBranch string,
) (int, error) {
	// Prefer gh; it understands fork workflows with --repo when needed.
	repo, err := ghRepoFromRemote(ctx, r, baseRemote)
	if err != nil {
		return 0, err
	}
	if existingPR > 0 {
		// Update title/body only.
		args := []string{"pr", "edit", strconv.Itoa(existingPR),
			"--repo", repo,
			"--title", title,
			"--body", body,
		}
		if err := r.Run(ctx, "gh", args...); err != nil {
			return existingPR, fmt.Errorf("gh pr edit #%d: %w", existingPR, err)
		}
		return existingPR, nil
	}
	// Create: for fork → upstream PRs gh needs the head as forkOwner:branch;
	// a bare branch name only works when the PR stays within one repo.
	head := branch
	if baseRemote == "upstream" {
		if owner, err := ghOwnerFromRemote(ctx, r, "origin"); err == nil && owner != "" {
			head = owner + ":" + branch
		}
	}
	out, err := r.Output(ctx, "gh", "pr", "create",
		"--repo", repo,
		"--base", baseBranch,
		"--head", head,
		"--title", title,
		"--body", body,
	)
	if err != nil {
		return 0, fmt.Errorf("gh pr create: %w", err)
	}
	return parsePRNumber(string(out))
}

func ghRepoFromRemote(ctx context.Context, r Runner, remote string) (string, error) {
	raw, err := RemoteURL(ctx, r, remote)
	if err != nil {
		return "", err
	}
	return parseOwnerRepo(raw)
}

func ghOwnerFromRemote(ctx context.Context, r Runner, remote string) (string, error) {
	or, err := ghRepoFromRemote(ctx, r, remote)
	if err != nil {
		return "", err
	}
	owner, _, _ := strings.Cut(or, "/")
	return owner, nil
}

func parseOwnerRepo(remote string) (string, error) {
	u, err := HTTPBrowseURL(remote)
	if err != nil {
		return "", err
	}
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	// host/owner/repo
	parts := strings.Split(u, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("cannot parse owner/repo from %q", remote)
	}
	return parts[1] + "/" + parts[2], nil
}

func parsePRNumber(out string) (int, error) {
	// gh prints URL like https://github.com/o/r/pull/42
	out = strings.TrimSpace(out)
	if i := strings.LastIndex(out, "/pull/"); i >= 0 {
		n, err := strconv.Atoi(strings.TrimSpace(out[i+len("/pull/"):]))
		if err == nil {
			return n, nil
		}
	}
	// sometimes just the number
	if n, err := strconv.Atoi(out); err == nil {
		return n, nil
	}
	return 0, fmt.Errorf("could not parse PR number from gh output %q", out)
}

// MergeMap builds branch→merged for stack map rendering.
// Only RelMerged (tip strictly behind trunk) counts — RelIdentical is an
// empty layer still based on trunk, not landed work.
func MergeMap(rows []LayerStatus) map[string]bool {
	m := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.Relation == RelMerged {
			m[row.Branch] = true
		}
	}
	return m
}

// PRURLBuilder returns a function that turns a PR number into a full URL
// for the base repo, or empty if unknown.
func PRURLBuilder(ctx context.Context, r Runner, baseRemote string) func(int) string {
	base, err := BrowseURLForRemote(ctx, r, baseRemote)
	if err != nil {
		return func(int) string { return "" }
	}
	return func(n int) string {
		if n <= 0 {
			return ""
		}
		return fmt.Sprintf("%s/pull/%d", strings.TrimSuffix(base, "/"), n)
	}
}
