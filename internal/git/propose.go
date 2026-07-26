// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrAutoMergeUnavailable reports that GitHub's auto-merge feature is switched
// off in the base repository's settings, so PRs cannot be flagged to merge
// automatically.
var ErrAutoMergeUnavailable = errors.New("auto-merge is not enabled for this repository")

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

// PROptions describes the pull request CreateOrUpdatePR should end up with.
type PROptions struct {
	Branch     string // head branch
	ExistingPR int    // update this PR; 0 opens a new one
	Title      string
	Body       string
	BaseRemote string // remote holding the base repository
	BaseBranch string
	// Draft opens the pull request as a draft. It applies only when opening:
	// GitHub has no draft toggle on edit, so an existing PR leaves draft
	// through MarkPRReady instead.
	Draft bool
}

// CreateOrUpdatePR opens or updates a PR for opts.Branch against the base
// branch using gh, and returns the PR number.
func CreateOrUpdatePR(ctx context.Context, r Runner, opts PROptions) (int, error) {
	// Prefer gh; it understands fork workflows with --repo when needed.
	repo, err := ghRepoFromRemote(ctx, r, opts.BaseRemote)
	if err != nil {
		return 0, err
	}
	if opts.ExistingPR > 0 {
		// Update title/body only.
		args := []string{"pr", "edit", strconv.Itoa(opts.ExistingPR),
			"--repo", repo,
			"--title", opts.Title,
			"--body", opts.Body,
		}
		if err := r.Run(ctx, "gh", args...); err != nil {
			return opts.ExistingPR, fmt.Errorf("gh pr edit #%d: %w", opts.ExistingPR, err)
		}
		return opts.ExistingPR, nil
	}
	// Create: for fork → upstream PRs gh needs the head as forkOwner:branch;
	// a bare branch name only works when the PR stays within one repo.
	head := opts.Branch
	if opts.BaseRemote == "upstream" {
		if owner, err := ghOwnerFromRemote(ctx, r, "origin"); err == nil && owner != "" {
			head = owner + ":" + opts.Branch
		}
	}
	args := []string{"pr", "create",
		"--repo", repo,
		"--base", opts.BaseBranch,
		"--head", head,
		"--title", opts.Title,
		"--body", opts.Body,
	}
	if opts.Draft {
		args = append(args, "--draft")
	}
	out, err := r.Output(ctx, "gh", args...)
	if err != nil {
		return 0, fmt.Errorf("gh pr create: %w", err)
	}
	return parsePRNumber(string(out))
}

// MarkPRReady takes PR pr out of draft, so it can be reviewed and merged.
// Proposing is re-runnable, so it is idempotent: a PR that is already ready
// reports switched = false and is left untouched.
func MarkPRReady(ctx context.Context, r Runner, baseRemote string, pr int) (switched bool, err error) {
	repo, err := ghRepoFromRemote(ctx, r, baseRemote)
	if err != nil {
		return false, err
	}
	out, err := r.Output(ctx, "gh", "pr", "view", strconv.Itoa(pr),
		"--repo", repo, "--json", "isDraft", "--jq", ".isDraft")
	if err != nil {
		return false, fmt.Errorf("gh pr view #%d: %w", pr, err)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return false, nil
	}
	if err := r.Run(ctx, "gh", "pr", "ready", strconv.Itoa(pr), "--repo", repo); err != nil {
		return false, fmt.Errorf("gh pr ready #%d: %w", pr, err)
	}
	return true, nil
}

// PRMerged reports whether pull request pr has been merged in the base
// repository. It is the remote's answer to whether a layer's work has landed,
// which local history alone cannot give for a layer that landed by
// fast-forward: trunk moves up onto the branch, leaving the two identical.
func PRMerged(ctx context.Context, r Runner, baseRemote string, pr int) (bool, error) {
	repo, err := ghRepoFromRemote(ctx, r, baseRemote)
	if err != nil {
		return false, err
	}
	out, err := r.Output(ctx, "gh", "pr", "view", strconv.Itoa(pr),
		"--repo", repo, "--json", "state", "--jq", ".state")
	if err != nil {
		return false, fmt.Errorf("gh pr view #%d: %w", pr, err)
	}
	return strings.EqualFold(strings.TrimSpace(string(out)), "merged"), nil
}

// CheckAutoMerge verifies the base repository can honor an auto-merge with
// the given method ("merge", "rebase", or "squash"): ErrAutoMergeUnavailable
// when the repository has auto-merge switched off, and a descriptive error
// when the repository's settings disallow the method itself.
func CheckAutoMerge(ctx context.Context, r Runner, baseRemote, method string) error {
	repo, err := ghRepoFromRemote(ctx, r, baseRemote)
	if err != nil {
		return err
	}
	out, err := r.Output(ctx, "gh", "repo", "view", repo, "--json",
		"autoMergeAllowed,rebaseMergeAllowed,squashMergeAllowed,mergeCommitAllowed")
	if err != nil {
		return fmt.Errorf("gh repo view %s: %w", repo, err)
	}
	var allowed struct {
		AutoMerge   bool `json:"autoMergeAllowed"`
		RebaseMerge bool `json:"rebaseMergeAllowed"`
		SquashMerge bool `json:"squashMergeAllowed"`
		MergeCommit bool `json:"mergeCommitAllowed"`
	}
	if err := json.Unmarshal(out, &allowed); err != nil {
		return fmt.Errorf("parse gh repo view %s: %w", repo, err)
	}
	if !allowed.AutoMerge {
		return ErrAutoMergeUnavailable
	}
	methodAllowed := map[string]bool{
		"merge":  allowed.MergeCommit,
		"rebase": allowed.RebaseMerge,
		"squash": allowed.SquashMerge,
	}
	if !methodAllowed[method] {
		return fmt.Errorf("repository %s does not allow %s merges", repo, method)
	}
	return nil
}

// EnableAutoMerge flags PR pr to merge automatically with the given method
// (validated by CheckAutoMerge) once its requirements pass. Proposing is
// re-runnable, so enabling is idempotent: a PR whose auto-merge request is
// already pending reports already = true and is left untouched.
func EnableAutoMerge(ctx context.Context, r Runner, baseRemote string, pr int,
	method string) (already bool, err error) {
	repo, err := ghRepoFromRemote(ctx, r, baseRemote)
	if err != nil {
		return false, err
	}
	// A failed probe is not fatal — enabling below reports the real problem.
	out, err := r.Output(ctx, "gh", "pr", "view", strconv.Itoa(pr),
		"--repo", repo, "--json", "autoMergeRequest", "--jq", ".autoMergeRequest")
	if pending := strings.TrimSpace(string(out)); err == nil && pending != "" && pending != "null" {
		return true, nil
	}
	if _, err := r.Output(ctx, "gh", "pr", "merge", strconv.Itoa(pr),
		"--repo", repo, "--auto", "--"+method); err != nil {
		return false, fmt.Errorf("gh pr merge --auto #%d: %w", pr, err)
	}
	return false, nil
}

// autoMergeCommentBody is the comment convention merge bots watch for in
// repositories that gate merges themselves instead of using GitHub auto-merge.
const autoMergeCommentBody = "/auto-merge"

// AddAutoMergeComment posts a /auto-merge comment on PR pr for a merge bot to
// act on. Proposing is re-runnable and repeated comments would re-trigger the
// bot, so a PR that already carries the comment is left untouched
// (added = false).
func AddAutoMergeComment(ctx context.Context, r Runner, baseRemote string, pr int) (added bool, err error) {
	repo, err := ghRepoFromRemote(ctx, r, baseRemote)
	if err != nil {
		return false, err
	}
	// A failed probe is not fatal — worst case the comment posts again.
	out, err := r.Output(ctx, "gh", "pr", "view", strconv.Itoa(pr),
		"--repo", repo, "--json", "comments", "--jq", ".comments[].body")
	if err == nil {
		for line := range strings.Lines(string(out)) {
			if strings.TrimSpace(line) == autoMergeCommentBody {
				return false, nil
			}
		}
	}
	if _, err := r.Output(ctx, "gh", "pr", "comment", strconv.Itoa(pr),
		"--repo", repo, "--body", autoMergeCommentBody); err != nil {
		return false, fmt.Errorf("gh pr comment #%d: %w", pr, err)
	}
	return true, nil
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
