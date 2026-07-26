// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// ghFakeRunner cans Output results keyed by the full command line
// ("name arg arg ..."), recording every invocation — Run and Output alike —
// for order/absence checks. A key in errs fails that command.
type ghFakeRunner struct {
	outputs map[string]string
	errs    map[string]error
	calls   []string
}

func (f *ghFakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	key := cmdKey(name, args)
	f.calls = append(f.calls, key)
	if err, ok := f.errs[key]; ok {
		return nil, err
	}
	out, ok := f.outputs[key]
	if !ok {
		return nil, fmt.Errorf("unexpected command %q", key)
	}
	return []byte(out), nil
}

func (f *ghFakeRunner) Run(_ context.Context, name string, args ...string) error {
	key := cmdKey(name, args)
	f.calls = append(f.calls, key)
	return f.errs[key]
}

func (f *ghFakeRunner) RunInteractive(context.Context, string, ...string) error { return nil }

func cmdKey(name string, args []string) string {
	return strings.Join(append([]string{name}, args...), " ")
}

const (
	fakeRemoteURLCmd       = "git remote get-url upstream"
	fakeOriginRemoteURLCmd = "git remote get-url origin"
	fakeRepoViewCmd        = "gh repo view acme/widgets --json " +
		"autoMergeAllowed,rebaseMergeAllowed,squashMergeAllowed,mergeCommitAllowed"
	fakeRemoteURL = "https://github.com/acme/widgets.git\n"
)

func TestCreateOrUpdatePR(t *testing.T) {
	const (
		createCmd = "gh pr create --repo acme/widgets --base main --head feat-a " +
			"--title subject --body body"
		draftCreateCmd = createCmd + " --draft"
		editCmd        = "gh pr edit 7 --repo acme/widgets --title subject --body body"
	)
	base := PROptions{
		Branch:     "feat-a",
		Title:      "subject",
		Body:       "body",
		BaseRemote: "origin",
		BaseBranch: "main",
	}
	cases := []struct {
		name    string
		opts    PROptions
		wantPR  int
		wantCmd string
		absent  string
	}{
		{"opens a PR", base, 12, createCmd, draftCreateCmd},
		{"opens a draft PR", PROptions{Draft: true}, 12, draftCreateCmd, createCmd},
		{"edits an existing PR", PROptions{ExistingPR: 7}, 7, editCmd, createCmd},
		// gh pr edit cannot toggle draft, so an update ignores the flag.
		{"draft does not reach an update", PROptions{ExistingPR: 7, Draft: true}, 7, editCmd, draftCreateCmd},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts := base
			opts.ExistingPR = c.opts.ExistingPR
			opts.Draft = c.opts.Draft
			fake := &ghFakeRunner{outputs: map[string]string{
				fakeOriginRemoteURLCmd: fakeRemoteURL,
				createCmd:              "https://github.com/acme/widgets/pull/12\n",
				draftCreateCmd:         "https://github.com/acme/widgets/pull/12\n",
			}}
			pr, err := CreateOrUpdatePR(context.Background(), fake, opts)
			if err != nil {
				t.Fatalf("CreateOrUpdatePR() error: %v", err)
			}
			if pr != c.wantPR {
				t.Errorf("CreateOrUpdatePR() = %d, want %d", pr, c.wantPR)
			}
			if !slices.Contains(fake.calls, c.wantCmd) {
				t.Errorf("calls = %q, want %q", fake.calls, c.wantCmd)
			}
			if slices.Contains(fake.calls, c.absent) {
				t.Errorf("calls = %q, want %q absent", fake.calls, c.absent)
			}
		})
	}
}

func TestFindOpenPR(t *testing.T) {
	const (
		listCmd = "gh pr list --repo acme/widgets --head feat-a --base main " +
			"--state open --json number,headRepositoryOwner"
		forkRemoteURL = "https://github.com/deavon/widgets.git\n"
	)
	// The base repo owns the head in a single-remote repo; the fork behind
	// origin owns it when PRs target an upstream.
	pr := func(n int, owner string) string {
		return fmt.Sprintf(`{"number":%d,"headRepositoryOwner":{"login":%q}}`, n, owner)
	}
	cases := []struct {
		name       string
		baseRemote string
		originURL  string
		list       string
		listErr    error
		want       int
		wantErr    bool
	}{
		{
			name:       "adopts an open PR in a single-remote repo",
			baseRemote: "origin",
			originURL:  fakeRemoteURL,
			list:       "[" + pr(96, "acme") + "]",
			want:       96,
		},
		{
			name:       "adopts the fork's PR when proposing to an upstream",
			baseRemote: "upstream",
			originURL:  forkRemoteURL,
			list:       "[" + pr(96, "deavon") + "]",
			want:       96,
		},
		{
			name:       "ignores a same-named branch on another fork",
			baseRemote: "upstream",
			originURL:  forkRemoteURL,
			list:       "[" + pr(12, "someone-else") + "," + pr(96, "deavon") + "]",
			want:       96,
		},
		{
			name:       "reports none when the branch has no open PR",
			baseRemote: "origin",
			originURL:  fakeRemoteURL,
			list:       "[]",
			want:       0,
		},
		{
			name:       "surfaces the gh failure",
			baseRemote: "origin",
			originURL:  fakeRemoteURL,
			listErr:    errors.New("gh: could not reach github.com"),
			wantErr:    true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &ghFakeRunner{outputs: map[string]string{
				fakeRemoteURLCmd:       fakeRemoteURL,
				fakeOriginRemoteURLCmd: c.originURL,
				listCmd:                c.list,
			}}
			if c.listErr != nil {
				fake.errs = map[string]error{listCmd: c.listErr}
			}
			got, err := FindOpenPR(context.Background(), fake, c.baseRemote, "feat-a", "main")
			if (err != nil) != c.wantErr {
				t.Fatalf("FindOpenPR() error = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("FindOpenPR() = %d, want %d", got, c.want)
			}
		})
	}
}

func TestMarkPRReady(t *testing.T) {
	const (
		viewCmd  = "gh pr view 7 --repo acme/widgets --json isDraft --jq .isDraft"
		readyCmd = "gh pr ready 7 --repo acme/widgets"
	)
	cases := []struct {
		name         string
		isDraft      string
		viewErr      error
		wantSwitched bool
		wantErr      bool
		wantReadied  bool
	}{
		{"readies a draft", "true\n", nil, true, false, true},
		{"leaves a ready PR alone", "false\n", nil, false, false, false},
		{"surfaces the gh failure", "", errors.New("no such pull request"), false, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &ghFakeRunner{outputs: map[string]string{
				fakeRemoteURLCmd: fakeRemoteURL,
				viewCmd:          c.isDraft,
				readyCmd:         "",
			}}
			if c.viewErr != nil {
				fake.errs = map[string]error{viewCmd: c.viewErr}
			}
			switched, err := MarkPRReady(context.Background(), fake, "upstream", 7)
			if (err != nil) != c.wantErr {
				t.Fatalf("MarkPRReady() error = %v, wantErr %v", err, c.wantErr)
			}
			if switched != c.wantSwitched {
				t.Errorf("MarkPRReady() switched = %v, want %v", switched, c.wantSwitched)
			}
			if readied := slices.Contains(fake.calls, readyCmd); readied != c.wantReadied {
				t.Errorf("gh pr ready invoked = %v, want %v", readied, c.wantReadied)
			}
		})
	}
}

func TestPRMerged(t *testing.T) {
	const viewCmd = "gh pr view 77 --repo acme/widgets --json state --jq .state"
	cases := []struct {
		name    string
		state   string
		viewErr error
		want    bool
		wantErr bool
	}{
		{"merged", "MERGED\n", nil, true, false},
		{"open", "OPEN\n", nil, false, false},
		{"closed without merging", "CLOSED\n", nil, false, false},
		{"gh failure", "", errors.New("gh: not found"), false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &ghFakeRunner{outputs: map[string]string{
				fakeRemoteURLCmd: fakeRemoteURL,
				viewCmd:          c.state,
			}}
			if c.viewErr != nil {
				fake.errs = map[string]error{viewCmd: c.viewErr}
			}
			got, err := PRMerged(context.Background(), fake, "upstream", 77)
			if (err != nil) != c.wantErr {
				t.Fatalf("PRMerged() error = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("PRMerged() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestCheckAutoMerge(t *testing.T) {
	const allAllowed = `{"autoMergeAllowed":true,"rebaseMergeAllowed":true,` +
		`"squashMergeAllowed":true,"mergeCommitAllowed":true}`
	cases := []struct {
		name       string
		repoJSON   string
		method     string
		wantIs     error  // matched with errors.Is when set
		wantErrSub string // substring match when set; both empty means success
	}{
		{"rebase allowed", allAllowed, "rebase", nil, ""},
		{"squash allowed", allAllowed, "squash", nil, ""},
		{"merge commit allowed", allAllowed, "merge", nil, ""},
		{
			name:     "auto-merge disabled is the sentinel",
			repoJSON: `{"autoMergeAllowed":false,"rebaseMergeAllowed":true,"squashMergeAllowed":true,"mergeCommitAllowed":true}`,
			method:   "rebase",
			wantIs:   ErrAutoMergeUnavailable,
		},
		{
			name: "disallowed method names the method",
			repoJSON: `{"autoMergeAllowed":true,"rebaseMergeAllowed":false,` +
				`"squashMergeAllowed":true,"mergeCommitAllowed":true}`,
			method:     "rebase",
			wantErrSub: "rebase",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &ghFakeRunner{outputs: map[string]string{
				fakeRemoteURLCmd: fakeRemoteURL,
				fakeRepoViewCmd:  c.repoJSON,
			}}
			err := CheckAutoMerge(context.Background(), fake, "upstream", c.method)
			switch {
			case c.wantIs != nil:
				if !errors.Is(err, c.wantIs) {
					t.Fatalf("CheckAutoMerge(%q) error = %v, want %v", c.method, err, c.wantIs)
				}
			case c.wantErrSub != "":
				if err == nil || !strings.Contains(err.Error(), c.wantErrSub) {
					t.Fatalf("CheckAutoMerge(%q) error = %v, want substring %q", c.method, err, c.wantErrSub)
				}
			case err != nil:
				t.Fatalf("CheckAutoMerge(%q) error: %v", c.method, err)
			}
		})
	}
}

func TestEnableAutoMerge(t *testing.T) {
	const (
		viewCmd  = "gh pr view 7 --repo acme/widgets --json autoMergeRequest --jq .autoMergeRequest"
		mergeCmd = "gh pr merge 7 --repo acme/widgets --auto --rebase"
	)
	cases := []struct {
		name        string
		pending     string
		mergeErr    error
		wantAlready bool
		wantErr     bool
		wantMerged  bool
	}{
		{"enables when no request pending", "null\n", nil, false, false, true},
		{"skips a pending request", `{"enabledAt":"2026-07-20T00:00:00Z"}` + "\n", nil, true, false, false},
		{"surfaces the gh failure", "null\n", errors.New("auto merge is not allowed"), false, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &ghFakeRunner{
				outputs: map[string]string{
					fakeRemoteURLCmd: fakeRemoteURL,
					viewCmd:          c.pending,
					mergeCmd:         "",
				},
			}
			if c.mergeErr != nil {
				fake.errs = map[string]error{mergeCmd: c.mergeErr}
			}
			already, err := EnableAutoMerge(context.Background(), fake, "upstream", 7, "rebase")
			if (err != nil) != c.wantErr {
				t.Fatalf("EnableAutoMerge() error = %v, wantErr %v", err, c.wantErr)
			}
			if already != c.wantAlready {
				t.Errorf("EnableAutoMerge() already = %v, want %v", already, c.wantAlready)
			}
			if merged := slices.Contains(fake.calls, mergeCmd); merged != c.wantMerged {
				t.Errorf("gh pr merge invoked = %v, want %v", merged, c.wantMerged)
			}
		})
	}
}

func TestAddAutoMergeComment(t *testing.T) {
	const (
		viewCmd    = "gh pr view 7 --repo acme/widgets --json comments --jq .comments[].body"
		commentCmd = "gh pr comment 7 --repo acme/widgets --body /auto-merge"
	)
	cases := []struct {
		name       string
		comments   string
		wantAdded  bool
		wantPosted bool
	}{
		{"posts on a PR without the comment", "looks good\n", true, true},
		{"posts when there are no comments", "", true, true},
		{"skips a PR already carrying the comment", "looks good\n/auto-merge\n", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &ghFakeRunner{outputs: map[string]string{
				fakeRemoteURLCmd: fakeRemoteURL,
				viewCmd:          c.comments,
				commentCmd:       "",
			}}
			added, err := AddAutoMergeComment(context.Background(), fake, "upstream", 7)
			if err != nil {
				t.Fatalf("AddAutoMergeComment() error: %v", err)
			}
			if added != c.wantAdded {
				t.Errorf("AddAutoMergeComment() added = %v, want %v", added, c.wantAdded)
			}
			if posted := slices.Contains(fake.calls, commentCmd); posted != c.wantPosted {
				t.Errorf("gh pr comment invoked = %v, want %v", posted, c.wantPosted)
			}
		})
	}
}
