// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"
	"testing"
)

func TestGroupCommits(t *testing.T) {
	c := func(subject string) Commit { return Commit{SHA: subject, Subject: subject} }
	tests := []struct {
		name       string
		subjects   []string
		autoSquash bool
		want       [][]string // subjects per group
	}{
		{
			name:       "no squash keeps one commit per group",
			subjects:   []string{"feat: a", "chore: tidy", "fix: b"},
			autoSquash: false,
			want:       [][]string{{"feat: a"}, {"chore: tidy"}, {"fix: b"}},
		},
		{
			name:       "chores fold into the commit below",
			subjects:   []string{"feat: a", "chore: tidy", "chore(deps): bump", "fix: b"},
			autoSquash: true,
			want:       [][]string{{"feat: a", "chore: tidy", "chore(deps): bump"}, {"fix: b"}},
		},
		{
			name:       "leading chore keeps its own group",
			subjects:   []string{"chore: scaffold", "chore: more", "feat: a"},
			autoSquash: true,
			want:       [][]string{{"chore: scaffold", "chore: more"}, {"feat: a"}},
		},
		{
			name:       "chore-like subjects that are not chores stay",
			subjects:   []string{"feat: a", "chores: not conventional", "chorex: nope"},
			autoSquash: true,
			want:       [][]string{{"feat: a"}, {"chores: not conventional"}, {"chorex: nope"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var commits []Commit
			for _, s := range tt.subjects {
				commits = append(commits, c(s))
			}
			groups := groupCommits(commits, tt.autoSquash)
			if len(groups) != len(tt.want) {
				t.Fatalf("groups = %d, want %d", len(groups), len(tt.want))
			}
			for i, g := range groups {
				if len(g) != len(tt.want[i]) {
					t.Fatalf("group %d = %d commits, want %d", i, len(g), len(tt.want[i]))
				}
				for j, cm := range g {
					if cm.Subject != tt.want[i][j] {
						t.Fatalf("group %d commit %d = %q, want %q", i, j, cm.Subject, tt.want[i][j])
					}
				}
			}
		})
	}
}

func TestIsChore(t *testing.T) {
	tests := []struct {
		subject string
		want    bool
	}{
		{"chore: tidy", true},
		{"chore(deps): bump x", true},
		{"chore!: drop config", true},
		{"chore(scope)!: drop", true},
		{"feat: a", false},
		{"chores: plural", false},
		{"chore missing colon", false},
		{"fix(chore): about chores", false},
	}
	for _, tt := range tests {
		if got := isChore(tt.subject); got != tt.want {
			t.Errorf("isChore(%q) = %v, want %v", tt.subject, got, tt.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		subject, want string
	}{
		{"feat(git): add expand command", "feat-git-add-expand-command"},
		{"Fix UPPER  case & symbols!!", "fix-upper-case-symbols"},
		{"---", ""},
		{"", ""},
		{strings.Repeat("very long subject ", 10), "very-long-subject-very-long-subject-very-long-su"},
	}
	for _, tt := range tests {
		got := slugify(tt.subject)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.subject, got, tt.want)
		}
		if len(got) > maxSlugLen+1 {
			t.Errorf("slugify(%q) too long: %d", tt.subject, len(got))
		}
	}
}

func TestLayerBranchNameFallsBackToSHA(t *testing.T) {
	m := newMem()
	ctx := context.Background()
	taken := map[string]bool{}
	unslugifiable := Commit{SHA: "0123456789abcdef", Subject: "!!!"}
	if name := layerBranchName(ctx, m, unslugifiable, taken); name != "commit-0123456" {
		t.Fatalf("fallback name = %q", name)
	}
	if name := layerBranchName(ctx, m, unslugifiable, taken); name != "commit-0123456-2" {
		t.Fatalf("deduplicated name = %q", name)
	}
}

// expandFixture builds main + a feat branch with feat/chore/feat commits and
// returns the runner. HEAD is left on the feature branch.
func expandFixture(t *testing.T) (*realGitRunner, Trunk) {
	t.Helper()
	r, _ := initRepo(t)
	gitRun(t, r.dir, "checkout", "-b", "feature")
	commitFile(t, r.dir, "a.txt", "a\n", "feat: first thing")
	commitFile(t, r.dir, "a_fmt.txt", "fmt\n", "chore: gofmt")
	commitFile(t, r.dir, "b.txt", "b\n", "feat: second thing")
	gitRun(t, r.dir, "update-ref", "refs/remotes/origin/main", "main")
	return r, Trunk{Remote: "origin", Branch: "main"}
}

func TestPlanExpand(t *testing.T) {
	r, trunk := expandFixture(t)
	ctx := t.Context()

	p, err := PlanExpand(ctx, r, trunk, "feature", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Layers) != 3 {
		t.Fatalf("layers = %+v", p.Layers)
	}
	if p.Layers[0].Branch != "feat-first-thing" || p.Layers[1].Branch != "chore-gofmt" {
		t.Fatalf("branch names = %q %q", p.Layers[0].Branch, p.Layers[1].Branch)
	}
	if p.Layers[2].Branch != "feature" {
		t.Fatalf("tip must keep the branch name, got %q", p.Layers[2].Branch)
	}
	if p.Squashes() {
		t.Fatal("no squashes expected without --auto-squash")
	}
	base, err := RevParse(ctx, r, "main")
	if err != nil || p.Base != base {
		t.Fatalf("base = %q, want main tip %q (%v)", p.Base, base, err)
	}

	p, err = PlanExpand(ctx, r, trunk, "feature", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Layers) != 2 || !p.Squashes() {
		t.Fatalf("auto-squash layers = %+v", p.Layers)
	}
	if len(p.Layers[0].Commits) != 2 || p.Layers[0].Commits[1].Subject != "chore: gofmt" {
		t.Fatalf("squash group = %+v", p.Layers[0].Commits)
	}
}

func TestPlanExpandRefusesTrunkAndShortBranches(t *testing.T) {
	r, trunk := expandFixture(t)
	ctx := t.Context()
	if _, err := PlanExpand(ctx, r, trunk, "main", false); err == nil {
		t.Fatal("expected refusal for trunk")
	}
	gitRun(t, r.dir, "checkout", "-b", "tiny", "main")
	commitFile(t, r.dir, "t.txt", "t\n", "feat: only one")
	if _, err := PlanExpand(ctx, r, trunk, "tiny", false); err == nil {
		t.Fatal("expected refusal for a single-commit branch")
	}
}

func TestExecuteExpandNoSquash(t *testing.T) {
	r, trunk := expandFixture(t)
	ctx := t.Context()
	p, err := PlanExpand(ctx, r, trunk, "feature", false)
	if err != nil {
		t.Fatal(err)
	}
	tipBefore, _ := RevParse(ctx, r, "feature")

	s, err := ExecuteExpand(ctx, r, p, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Layers) != 3 || s.Layers[2].Branch != "feature" {
		t.Fatalf("stack = %+v", s.Layers)
	}
	// No rewrite: layer branches point at the original commits.
	for i, l := range p.Layers[:2] {
		sha, err := RevParse(ctx, r, l.Branch)
		if err != nil || sha != l.Primary().SHA {
			t.Fatalf("layer %d branch %s at %q, want %q (%v)", i, l.Branch, sha, l.Primary().SHA, err)
		}
	}
	if tipAfter, _ := RevParse(ctx, r, "feature"); tipAfter != tipBefore {
		t.Fatalf("tip rewritten without squash: %q → %q", tipBefore, tipAfter)
	}
	if cur, _ := CurrentBranch(ctx, r); cur != "feature" {
		t.Fatalf("HEAD moved to %q", cur)
	}
	got, err := LoadStack(ctx, r, "feat-first-thing")
	if err != nil || got.ID != s.ID {
		t.Fatalf("stack not registered: %+v %v", got, err)
	}
}

func TestExecuteExpandAutoSquash(t *testing.T) {
	r, trunk := expandFixture(t)
	ctx := t.Context()
	p, err := PlanExpand(ctx, r, trunk, "feature", true)
	if err != nil {
		t.Fatal(err)
	}

	s, err := ExecuteExpand(ctx, r, p, "keepid42")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "keepid42" || len(s.Layers) != 2 {
		t.Fatalf("stack = %+v", s)
	}

	// The squashed bottom layer is a single commit carrying both diffs, with
	// the primary commit's message.
	bottom := s.Layers[0].Branch
	n, err := CommitsNotIn(ctx, r, bottom, "main")
	if err != nil || n != 1 {
		t.Fatalf("bottom layer commits = %d (%v), want 1", n, err)
	}
	subj, _ := CommitSubject(ctx, r, bottom)
	if subj != "feat: first thing" {
		t.Fatalf("squashed subject = %q", subj)
	}
	if err := r.Run(ctx, "git", "cat-file", "-e", bottom+":a_fmt.txt"); err != nil {
		t.Fatalf("chore change missing from squashed layer: %v", err)
	}

	// The tip was rewritten on top of the squash: two commits total, HEAD on
	// the feature branch, tree unchanged.
	if n, err = CommitsNotIn(ctx, r, "feature", "main"); err != nil || n != 2 {
		t.Fatalf("rewritten tip commits = %d (%v), want 2", n, err)
	}
	if cur, _ := CurrentBranch(ctx, r); cur != "feature" {
		t.Fatalf("HEAD on %q, want feature", cur)
	}
	for _, f := range []string{"a.txt", "a_fmt.txt", "b.txt"} {
		if err := r.Run(ctx, "git", "cat-file", "-e", "feature:"+f); err != nil {
			t.Fatalf("file %s missing from rewritten tip: %v", f, err)
		}
	}
}
