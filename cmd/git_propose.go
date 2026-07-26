// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var gitProposeFlags struct {
	All       bool
	AutoMerge autoMergeMode
	Browse    bool
	Copy      bool
	Draft     bool
}

// autoMergeMode is the --auto-merge value: a GitHub merge method ("merge",
// "rebase", "squash") for the native auto-merge feature, or "comment" for
// repositories where a merge bot watches for /auto-merge comments instead.
// The zero value means auto-merge was not requested. It implements
// pflag.Value; git-config defaults (dotty.propose.auto-merge) reach Set as
// the raw stored string, so Set is the single validation point.
type autoMergeMode string

const (
	autoMergeOff     autoMergeMode = ""
	autoMergeComment autoMergeMode = "comment"
)

// mergeMethod returns the gh merge method to enable, or "" when auto-merge is
// off or requested via comment.
func (m autoMergeMode) mergeMethod() string {
	if m == autoMergeOff || m == autoMergeComment {
		return ""
	}
	return string(m)
}

func (m *autoMergeMode) String() string { return string(*m) }

func (m *autoMergeMode) Type() string { return "mode" }

func (m *autoMergeMode) Set(v string) error {
	switch v := strings.ToLower(v); v {
	case "merge", "rebase", "squash", "comment":
		*m = autoMergeMode(v)
	default:
		return fmt.Errorf("invalid auto-merge mode %q: must be merge, rebase, squash, or comment", v)
	}
	return nil
}

var gitProposeCmd = &cobra.Command{
	Use:   "propose [--all] [--auto-merge=merge|rebase|squash|comment] [--browse] [--copy] [--draft]",
	Short: "Open or update trunk-based PRs for the stack.",
	Long: `Push stack branches and open pull requests against upstream/main
(or origin/main). Default: layers from the trunk through the current branch.
With --all, propose every layer in the stack.

A branch without stack lineage works too: propose adopts it first — as a
discovered chain when the local branch topology makes one obvious, otherwise
as a new single-layer stack.

Before anything is pushed the stack is reconciled with the remote: any layer
whose PR has already landed is dropped, its local and origin branches deleted
— ` + "`dotty git done`" + `'s cleanup, scoped to this stack and without leaving
your branch. Keep the branches with ` + "`git config set dotty.stack.cleanup false`" + `.

Every layer that remains must then be up to date with trunk
(fast-forwardable) and with the layers below it; if the stack has diverged or
a lower layer gained commits, you are prompted to rebase + resign, as
` + "`dotty git sync`" + ` does.

Each PR body includes a stack map with links. For multi-commit layers you pick
which commit supplies the title and description. A stacked layer owns its PR's
title and body: both are rewritten from that commit on every run, so change a
description by amending the commit, not by editing the PR on GitHub. A layer
whose PR dotty does not know about — opened by hand, or before the branch
joined a stack — adopts the open PR for that branch instead of colliding with
it, and that PR is dotty's from then on.

With --auto-merge=<merge|rebase|squash>, each proposed PR is flagged to merge
automatically with that method once its requirements pass. If the repository
has auto-merge switched off, or disallows the chosen method, propose warns and
continues. --auto-merge=comment instead posts a ` + "`/auto-merge`" + ` comment
on each PR, for repositories where a merge bot watches for it.

With --draft, each PR opens as a draft. Drafts cannot merge, so --draft and
--auto-merge are mutually exclusive and a configured auto-merge default is
ignored for the run. --draft applies when a PR is opened; an existing PR keeps
its draft state. Proposing again without --draft takes any draft PR out of
draft first, then applies auto-merge to it as usual.

With --browse, each proposed PR opens in your browser afterwards; with --copy,
the PR URLs (one per line) land on your clipboard. Make any of these the
default via git configuration: ` + "`git config set dotty.propose.browse true`" + `
(and dotty.propose.copy, dotty.propose.auto-merge, dotty.propose.draft).`,
	Example: `  dotty git propose
  dotty git propose --all
  dotty git propose --auto-merge=rebase
  dotty git propose --auto-merge=comment
  dotty git propose --draft
  dotty git propose --browse --copy`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ios := cli.System()
		r := newRunner(ios)
		ctx := cmd.Context()

		cur, err := git.CurrentBranch(ctx, r)
		if err != nil {
			return err
		}
		trunk, err := git.ResolveTrunk(ctx, r)
		if err != nil {
			return err
		}
		_ = git.FetchTrunk(ctx, r, trunk)

		s, err := git.LoadStack(ctx, r, cur)
		if errors.Is(err, git.ErrNotInStack) {
			s, err = adoptCurrentBranch(ctx, ios, r, trunk, cur)
		}
		if err != nil {
			return err
		}

		baseRemote, baseBranch, err := git.PRTarget(ctx, r)
		if err != nil {
			return err
		}
		prURL := git.PRURLBuilder(ctx, r, baseRemote)

		// The remote decides what is still worth proposing: a landed layer has
		// no commits left of its own, so it is dropped before the scope is
		// resolved rather than pushed and then rejected as empty.
		s, cur, err = pruneLandedLayers(ctx, ios, r, s, trunk, cur)
		if err != nil {
			return err
		}
		if len(s.Layers) == 0 {
			tui.Successf(ios, "Stack fully merged; nothing left to propose")
			return nil
		}

		through, err := git.ResolveProposeScope(s, cur, gitProposeFlags.All)
		if err != nil {
			return err
		}

		// A draft cannot merge, so --draft switches auto-merge off. Only a
		// configured default can reach here alongside it — the two flags are
		// mutually exclusive on the command line.
		autoMerge := gitProposeFlags.AutoMerge
		if gitProposeFlags.Draft && autoMerge != autoMergeOff {
			tui.Infof(ios, "Ignoring auto-merge (%s): draft pull requests do not merge", autoMerge)
			autoMerge = autoMergeOff
		}
		// Auto-merge support is repo-wide, so validate the chosen method once;
		// a repo that cannot honor it degrades to a warning, not a failure.
		if method := autoMerge.mergeMethod(); method != "" {
			err := git.CheckAutoMerge(ctx, r, baseRemote, method)
			switch {
			case errors.Is(err, git.ErrAutoMergeUnavailable):
				tui.Warnf(ios, "Auto-merge is not enabled for this repository; "+
					"enable it in the repository settings or use --auto-merge=comment")
				autoMerge = autoMergeOff
			case err != nil:
				tui.Warnf(ios, "Auto-merge unavailable: %v", err)
				autoMerge = autoMergeOff
			}
		}

		for i := range s.Layers {
			if s.Layers[i].TitleHint == "" {
				if subj, err := git.CommitSubject(ctx, r, s.Layers[i].Branch); err == nil {
					s.Layers[i].TitleHint = subj
				}
			}
		}

		rows := git.Status(ctx, r, s, trunk, cur)
		merged := git.MergeMap(rows)

		// PRs land by fast-forward, so every proposed layer must descend from
		// the trunk tip — and from the layer below it, which a mid-stack
		// commit breaks without any tip diverging from trunk. A stale or
		// diverged stack is rebased (and re-signed) first.
		scoped := rows[:through+1]
		if git.AnyDiverged(scoped) || git.AnyStale(ctx, r, scoped) {
			ok, cerr := tui.ConfirmDefault(ios,
				"Stack needs a rebase before proposing. Rebase + resign now?",
				"PRs must fast-forward onto "+trunk.Ref()+"; rewrites SHAs and re-signs with your hardware key.",
				true)
			if cerr != nil {
				if errors.Is(cerr, tui.ErrNotInteractive) {
					return errors.New("stack needs a rebase; run `dotty git sync --yes` first")
				}
				return cerr
			}
			if !ok {
				return errors.New("stack needs a rebase; PRs must be fast-forwardable (run `dotty git sync`)")
			}
			// rebaseResignStack returns HEAD to cur when it finishes.
			if err := rebaseResignStack(ctx, ios, r, s, trunk); err != nil {
				return err
			}
			rows = git.Status(ctx, r, s, trunk, cur)
			merged = git.MergeMap(rows)
		}

		var proposedURLs []string
		for i := 0; i <= through; i++ {
			layer := &s.Layers[i]
			if merged[layer.Branch] {
				tui.Infof(ios, "Skipping %s (already on trunk)", layer.Branch)
				continue
			}
			if err := git.PushBranch(ctx, r, layer.Branch); err != nil {
				return fmt.Errorf("push %s: %w", layer.Branch, err)
			}

			parent := git.ParentRevForLayer(s, i, trunk)
			commits, err := git.LayerCommits(ctx, r, parent, layer.Branch)
			if err != nil {
				return err
			}
			if len(commits) == 0 {
				return fmt.Errorf("layer %s has no commits of its own: commit something, "+
					"or run `dotty git sync` if its pull request has landed", layer.Branch)
			}

			var chosen git.Commit
			if len(commits) == 1 {
				chosen = commits[0]
			} else {
				opts := make([]tui.Option, len(commits))
				for j, c := range commits {
					short := c.SHA[:min(len(c.SHA), 7)]
					opts[j] = tui.Option{
						Label: fmt.Sprintf("%s %s", short, c.Subject),
						Value: c.SHA,
					}
				}
				sha, err := tui.FuzzySelect(ios,
					fmt.Sprintf("PR title/body commit for %s", layer.Branch), opts)
				if err != nil {
					return err
				}
				if j := slices.IndexFunc(commits, func(c git.Commit) bool { return c.SHA == sha }); j >= 0 {
					chosen = commits[j]
				}
			}

			layer.TitleSHA = chosen.SHA
			layer.TitleHint = chosen.Subject

			existingPR := layer.PR
			if existingPR == 0 {
				// A PR opened outside dotty — or before this branch joined a
				// stack — is invisible in local config, and GitHub refuses a
				// second one for the same head. Adopt it instead of colliding.
				found, ferr := git.FindOpenPR(ctx, r, baseRemote, layer.Branch, baseBranch)
				switch {
				case ferr != nil:
					tui.Warnf(ios, "Could not look for an existing PR on %s: %v", layer.Branch, ferr)
				case found > 0:
					tui.Infof(ios, "Adopted untracked PR#%d for %s", found, layer.Branch)
					existingPR = found
				}
			}

			stackMD := git.FormatStackMap(s, layer.Branch, prURL, merged)
			title := chosen.Subject
			body := git.BuildPRBody(s.ID, stackMD, chosen.Body)

			n, err := git.CreateOrUpdatePR(ctx, r, git.PROptions{
				Branch:     layer.Branch,
				ExistingPR: existingPR,
				Title:      title,
				Body:       body,
				BaseRemote: baseRemote,
				BaseBranch: baseBranch,
				Draft:      gitProposeFlags.Draft,
			})
			if err != nil {
				return err
			}
			layer.PR = n
			kind := "PR"
			if gitProposeFlags.Draft && existingPR == 0 {
				kind = "draft PR"
			}
			tui.Successf(ios, "Proposed %s → %s#%d (%s)", layer.Branch, kind, n, title)

			// Only a PR that already existed can be a draft to clear — one just
			// opened without --draft is ready already. This precedes auto-merge:
			// GitHub will not flag a draft to merge.
			if existingPR > 0 && !gitProposeFlags.Draft {
				switched, err := git.MarkPRReady(ctx, r, baseRemote, n)
				switch {
				case err != nil:
					tui.Warnf(ios, "Could not take PR#%d out of draft: %v", n, err)
				case switched:
					tui.Infof(ios, "PR#%d is out of draft and ready for review", n)
				}
			}

			if autoMerge == autoMergeComment {
				added, err := git.AddAutoMergeComment(ctx, r, baseRemote, n)
				switch {
				case err != nil:
					tui.Warnf(ios, "Could not comment /auto-merge on PR#%d: %v", n, err)
				case added:
					tui.Infof(ios, "Requested auto-merge on PR#%d via /auto-merge comment", n)
				}
			} else if method := autoMerge.mergeMethod(); method != "" {
				already, err := git.EnableAutoMerge(ctx, r, baseRemote, n, method)
				switch {
				case err != nil:
					tui.Warnf(ios, "Could not set PR#%d to auto-merge: %v", n, err)
				case !already:
					tui.Infof(ios, "PR#%d will auto-merge (%s) once requirements pass", n, method)
				}
			}

			if u := prURL(n); u != "" {
				proposedURLs = append(proposedURLs, u)
			}
		}

		if err := git.SaveStack(ctx, r, s); err != nil {
			return err
		}
		// Second pass: every stack map now knows every layer's PR number.
		if err := refreshOpenPRBodies(ctx, ios, r, s, trunk); err != nil {
			return err
		}

		if (gitProposeFlags.Browse || gitProposeFlags.Copy) && len(proposedURLs) == 0 {
			tui.Warnf(ios, "No PR URLs to open or copy")
			return nil
		}
		if gitProposeFlags.Copy {
			if err := cli.CopyToClipboard(ctx, strings.Join(proposedURLs, "\n")); err != nil {
				return err
			}
			tui.Infof(ios, "Copied %d PR URL(s) to the clipboard", len(proposedURLs))
		}
		if gitProposeFlags.Browse {
			for _, u := range proposedURLs {
				if err := git.OpenBrowser(u); err != nil {
					return err
				}
			}
		}
		return nil
	},
}

// adoptCurrentBranch gives a branch with no recorded lineage a stack to
// propose from: an obvious local chain when discovery finds one, otherwise a
// new single-layer stack holding just this branch.
func adoptCurrentBranch(ctx context.Context, ios cli.IOStreams, r *cli.ExecRunner,
	trunk git.Trunk, branch string,
) (git.Stack, error) {
	if branch == trunk.Branch {
		return git.Stack{}, fmt.Errorf("refusing to propose trunk branch %q", branch)
	}
	s, ok, err := git.DiscoverStack(ctx, r, trunk, branch)
	if err != nil {
		return git.Stack{}, err
	}
	if ok {
		if err := git.SaveStack(ctx, r, s); err != nil {
			return git.Stack{}, fmt.Errorf("save discovered stack: %w", err)
		}
		tui.Infof(ios, "Discovered a stack of %d layers containing %s", len(s.Layers), branch)
		return s, nil
	}
	s, err = git.AdoptBranch(ctx, r, branch)
	if err != nil {
		return git.Stack{}, err
	}
	tui.Infof(ios, "Registered %s as a single-layer stack", branch)
	return s, nil
}

func init() {
	gitProposeCmd.Flags().BoolVar(&gitProposeFlags.All, "all", false,
		"propose every layer in the stack, not only through the current branch")
	gitProposeCmd.Flags().Var(&gitProposeFlags.AutoMerge, "auto-merge",
		"auto-merge each proposed pull request with the given method (merge, rebase, or squash); "+
			"mode comment posts a /auto-merge comment for a merge bot instead")
	gitProposeCmd.Flags().BoolVar(&gitProposeFlags.Browse, "browse", false,
		"open each proposed pull request in the browser")
	gitProposeCmd.Flags().BoolVar(&gitProposeFlags.Copy, "copy", false,
		"copy the proposed pull request URL(s) to the clipboard")
	gitProposeCmd.Flags().BoolVar(&gitProposeFlags.Draft, "draft", false,
		"open each pull request as a draft; proposing again without it readies them")
	// A draft cannot merge. Configured defaults never mark a flag as changed,
	// so this refuses the pair on the command line only — dotty.propose.draft
	// alongside dotty.propose.auto-merge is resolved in favour of the draft.
	gitProposeCmd.MarkFlagsMutuallyExclusive("draft", "auto-merge")
	gitCmd.AddCommand(gitProposeCmd)
}
