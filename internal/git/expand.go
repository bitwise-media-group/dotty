// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
)

// ExpandLayer is one planned layer of `git expand`: the branch to create and
// the commits it will carry, oldest first. A layer with more than one commit
// is a squash group — the first commit donates the branch name, message, and
// author; the chore commits after it fold in.
type ExpandLayer struct {
	Branch  string
	Commits []Commit
}

// Primary is the commit that names the layer and supplies its message.
func (l ExpandLayer) Primary() Commit { return l.Commits[0] }

// ExpandPlan describes the stack `git expand` would create from a branch.
// It is a pure description: nothing changes until ExecuteExpand.
type ExpandPlan struct {
	Branch string        // the branch being expanded; stays as the tip layer
	Base   string        // SHA the first commit sits on (replay start point)
	Layers []ExpandLayer // bottom → tip
}

// Squashes reports whether any layer folds more than one commit, which
// rewrites history from that layer upward.
func (p ExpandPlan) Squashes() bool {
	for _, l := range p.Layers {
		if len(l.Commits) > 1 {
			return true
		}
	}
	return false
}

// PlanExpand plans one stack layer per commit unique to branch (trunk..HEAD),
// with the branch itself staying as the tip layer. With autoSquash, each
// chore commit folds into the layer below it instead of getting its own
// layer; a chore that is the very first commit has no layer below and keeps
// its own. The plan is not applied and no refs change.
func PlanExpand(ctx context.Context, r Runner, trunk Trunk, branch string, autoSquash bool) (ExpandPlan, error) {
	if branch == trunk.Branch {
		return ExpandPlan{}, fmt.Errorf("refusing to expand trunk branch %q", branch)
	}
	if err := ensureBranchName(branch); err != nil {
		return ExpandPlan{}, err
	}
	merges, err := r.Output(ctx, "git", "rev-list", "--count", "--merges", trunk.Ref()+".."+branch)
	if err != nil {
		return ExpandPlan{}, fmt.Errorf("count merges on %s: %w", branch, err)
	}
	if n, err := strconv.Atoi(strings.TrimSpace(string(merges))); err == nil && n > 0 {
		return ExpandPlan{}, fmt.Errorf("%s contains %d merge commit(s); expand needs a linear history", branch, n)
	}
	commits, err := LayerCommits(ctx, r, trunk.Ref(), branch)
	if err != nil {
		return ExpandPlan{}, err
	}
	if len(commits) < 2 {
		return ExpandPlan{}, fmt.Errorf("%s has %d commit(s) not on %s; nothing to expand",
			branch, len(commits), trunk.Ref())
	}
	base, err := RevParse(ctx, r, commits[0].SHA+"^")
	if err != nil {
		return ExpandPlan{}, err
	}
	groups := groupCommits(commits, autoSquash)
	p := ExpandPlan{Branch: branch, Base: base, Layers: make([]ExpandLayer, len(groups))}
	taken := map[string]bool{branch: true}
	for i, g := range groups {
		name := branch // the expanded branch keeps its name as the tip layer
		if i < len(groups)-1 {
			name = layerBranchName(ctx, r, g[0], taken)
		}
		p.Layers[i] = ExpandLayer{Branch: name, Commits: g}
	}
	return p, nil
}

// groupCommits splits commits (oldest first) into one group per layer. With
// autoSquash a chore commit joins the group below it; the first commit always
// starts a group because there is nothing below it to fold into.
func groupCommits(commits []Commit, autoSquash bool) [][]Commit {
	var groups [][]Commit
	for _, c := range commits {
		if autoSquash && isChore(c.Subject) && len(groups) > 0 {
			groups[len(groups)-1] = append(groups[len(groups)-1], c)
			continue
		}
		groups = append(groups, []Commit{c})
	}
	return groups
}

// choreSubject matches Conventional Commit chore subjects:
// "chore: …", "chore(scope): …", and the breaking "chore!: …" forms.
var choreSubject = regexp.MustCompile(`^chore(\([^)]*\))?!?:`)

func isChore(subject string) bool { return choreSubject.MatchString(subject) }

// layerBranchName derives a unique branch name from a commit: its subject
// slugified (falling back to the short SHA when the subject has no usable
// characters), then deduplicated against existing local branches and names
// already claimed by this plan.
func layerBranchName(ctx context.Context, r Runner, c Commit, taken map[string]bool) string {
	slug := slugify(c.Subject)
	if slug == "" {
		slug = "commit-" + c.SHA[:min(len(c.SHA), 7)]
	}
	name := slug
	for n := 2; taken[name] || branchExists(ctx, r, name); n++ {
		name = fmt.Sprintf("%s-%d", slug, n)
	}
	taken[name] = true
	return name
}

func branchExists(ctx context.Context, r Runner, name string) bool {
	_, err := RevParse(ctx, r, "refs/heads/"+name)
	return err == nil
}

// maxSlugLen caps generated branch names so a long subject stays readable.
const maxSlugLen = 48

// slugify turns a commit subject into a branch-safe slug: lowercase
// alphanumerics with single dashes between words. It returns "" when the
// subject has no usable characters — the caller picks a meaningful fallback.
func slugify(subject string) string {
	var b strings.Builder
	dash := true // suppress a leading dash
	for _, c := range strings.ToLower(subject) {
		switch {
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			b.WriteRune(c)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= maxSlugLen {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

// FormatExpandPlan writes the planned stack, bottom layer first, with squash
// group members indented under the commit they fold into.
func FormatExpandPlan(w io.Writer, p ExpandPlan, trunk Trunk) {
	_, _ = fmt.Fprintf(w, "planned stack for %s · trunk %s\n", p.Branch, trunk.Ref())
	tw := tabwriter.NewWriter(w, 0, 1, 2, ' ', 0)
	for i, l := range p.Layers {
		c := l.Primary()
		_, _ = fmt.Fprintf(tw, "[%d]\t%s\t%s\t%s\n", i+1, l.Branch, c.SHA[:min(len(c.SHA), 7)], c.Subject)
		for _, sq := range l.Commits[1:] {
			_, _ = fmt.Fprintf(tw, "\t  + squash\t%s\t%s\n", sq.SHA[:min(len(sq.SHA), 7)], sq.Subject)
		}
	}
	_ = tw.Flush()
}

// IsWorktreeClean reports whether the working tree and index have no pending
// changes (untracked files included).
func IsWorktreeClean(ctx context.Context, r Runner) (bool, error) {
	out, err := r.Output(ctx, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(string(out)) == "", nil
}

// ExecuteExpand applies the plan: creates the layer branches and registers
// the stack. Layers below the first squash keep their existing (already
// signed) commits and only gain a branch ref. From the first squash onward,
// commits are replayed with cherry-pick and committed normally — so
// commit.gpgsign still applies and rewritten commits are re-signed — and the
// expanded branch is reset to the rewritten tip. reuseID keeps an existing
// stack id when the branch was already a registered single-layer stack;
// empty generates a new one. On failure it restores the original branch and
// deletes any layer branches it created.
func ExecuteExpand(ctx context.Context, r Runner, p ExpandPlan, reuseID string) (Stack, error) {
	if len(p.Layers) == 0 {
		return Stack{}, fmt.Errorf("empty expand plan for %s", p.Branch)
	}
	id := reuseID
	if id == "" {
		var err error
		if id, err = newStackID(); err != nil {
			return Stack{}, err
		}
	}
	s := Stack{ID: id, Layers: make([]Layer, 0, len(p.Layers))}

	origTip, err := RevParse(ctx, r, p.Branch)
	if err != nil {
		return Stack{}, err
	}
	var created []string
	fail := func(err error) (Stack, error) {
		// Best-effort restore: drop any half-applied pick, put the expanded
		// branch back on its original tip with HEAD on it, and remove the
		// layer branches created so far.
		_ = r.Run(ctx, "git", "cherry-pick", "--abort")
		_ = r.Run(ctx, "git", "checkout", "-B", p.Branch, origTip)
		for _, b := range created {
			_ = r.Run(ctx, "git", "branch", "-D", b)
		}
		return Stack{}, err
	}

	// first is the lowest layer whose commits must be rewritten; everything
	// below it keeps its original SHAs and only needs a branch ref.
	first := firstSquashIndex(p.Layers)

	for i := 0; i < first && i < len(p.Layers)-1; i++ {
		l := p.Layers[i] // single commit: squash groups start at index first
		if err := r.Run(ctx, "git", "branch", l.Branch, l.Primary().SHA); err != nil {
			return fail(fmt.Errorf("create branch %s at %s: %w", l.Branch, l.Primary().SHA, err))
		}
		created = append(created, l.Branch)
		s.Layers = append(s.Layers, Layer{Branch: l.Branch, TitleSHA: l.Primary().SHA, TitleHint: l.Primary().Subject})
	}

	if first == len(p.Layers) {
		// No rewrites; the expanded branch already points at the tip commit.
		tip := p.Layers[len(p.Layers)-1].Primary()
		s.Layers = append(s.Layers, Layer{Branch: p.Branch, TitleSHA: tip.SHA, TitleHint: tip.Subject})
		if err := saveStack(ctx, r, s); err != nil {
			return fail(err)
		}
		return s, nil
	}

	// Replay from the last untouched commit (or the base below the first
	// commit). Each group re-applies the original diffs in order onto the
	// commit just created, so the picks are conflict-free by construction.
	start := p.Base
	if first > 0 {
		start = p.Layers[first-1].Primary().SHA
	}
	if err := r.Run(ctx, "git", "checkout", "--detach", start); err != nil {
		return fail(fmt.Errorf("checkout %s: %w", start, err))
	}
	for i := first; i < len(p.Layers); i++ {
		l := p.Layers[i]
		sha, err := commitLayer(ctx, r, l)
		if err != nil {
			return fail(err)
		}
		if i < len(p.Layers)-1 {
			if err := r.Run(ctx, "git", "branch", l.Branch, sha); err != nil {
				return fail(fmt.Errorf("create branch %s at %s: %w", l.Branch, sha, err))
			}
			created = append(created, l.Branch)
		} else if err := r.Run(ctx, "git", "checkout", "-B", p.Branch, sha); err != nil {
			return fail(fmt.Errorf("reset %s to rewritten tip %s: %w", p.Branch, sha, err))
		}
		s.Layers = append(s.Layers, Layer{Branch: l.Branch, TitleSHA: sha, TitleHint: l.Primary().Subject})
	}
	if err := saveStack(ctx, r, s); err != nil {
		return fail(err)
	}
	return s, nil
}

// firstSquashIndex returns the index of the lowest layer that folds more than
// one commit, or len(layers) when no layer squashes.
func firstSquashIndex(layers []ExpandLayer) int {
	for i, l := range layers {
		if len(l.Commits) > 1 {
			return i
		}
	}
	return len(layers)
}

// commitLayer replays one layer's commits onto HEAD and returns the resulting
// commit SHA. A single commit is cherry-picked directly; a squash group is
// picked with --no-commit and committed once, reusing the primary commit's
// message and author. Commit-creating steps run interactively so the signing
// program can prompt for the hardware key.
func commitLayer(ctx context.Context, r Runner, l ExpandLayer) (string, error) {
	if len(l.Commits) == 1 {
		if err := r.RunInteractive(ctx, "git", "cherry-pick", "--allow-empty", l.Primary().SHA); err != nil {
			return "", fmt.Errorf("cherry-pick %s: %w", l.Primary().SHA, err)
		}
	} else {
		args := []string{"cherry-pick", "--no-commit"}
		for _, c := range l.Commits {
			args = append(args, c.SHA)
		}
		if err := r.Run(ctx, "git", args...); err != nil {
			return "", fmt.Errorf("squash picks for %s: %w", l.Branch, err)
		}
		if err := r.RunInteractive(ctx, "git", "commit", "--allow-empty", "-C", l.Primary().SHA); err != nil {
			return "", fmt.Errorf("squash commit for %s: %w", l.Branch, err)
		}
	}
	return RevParse(ctx, r, "HEAD")
}
