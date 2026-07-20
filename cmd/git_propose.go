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
	All    bool
	Browse bool
	Copy   bool
}

var gitProposeCmd = &cobra.Command{
	Use:   "propose [--all] [--browse] [--copy]",
	Short: "Open or update trunk-based PRs for the stack.",
	Long: `Push stack branches and open pull requests against upstream/main
(or origin/main). Default: layers from the trunk through the current branch.
With --all, propose every layer in the stack.

A branch without stack lineage works too: propose adopts it first — as a
discovered chain when the local branch topology makes one obvious, otherwise
as a new single-layer stack. Before any PR opens, every proposed layer must
be up to date with trunk (fast-forwardable) and with the layers below it; if
the stack has diverged or a lower layer gained commits, you are prompted to
rebase + resign, as ` + "`dotty git sync`" + ` does.

Each PR body includes a stack map with links. For multi-commit layers you pick
which commit supplies the title and description.

With --browse, each proposed PR opens in your browser afterwards; with --copy,
the PR URLs (one per line) land on your clipboard.`,
	Example: `  dotty git propose
  dotty git propose --all
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
		through, err := git.ResolveProposeScope(s, cur, gitProposeFlags.All)
		if err != nil {
			return err
		}

		baseRemote, baseBranch, err := git.PRTarget(ctx, r)
		if err != nil {
			return err
		}
		prURL := git.PRURLBuilder(ctx, r, baseRemote)

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
				return fmt.Errorf("layer %s has no commits unique to this layer", layer.Branch)
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

			stackMD := git.FormatStackMap(s, layer.Branch, prURL, merged)
			body := git.BuildPRBody(s.ID, stackMD, chosen.Body)
			title := chosen.Subject

			n, err := git.CreateOrUpdatePR(ctx, r, layer.Branch, layer.PR, title, body, baseRemote, baseBranch)
			if err != nil {
				return err
			}
			layer.PR = n
			tui.Successf(ios, "Proposed %s → PR#%d (%s)", layer.Branch, n, title)
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
	gitProposeCmd.Flags().BoolVar(&gitProposeFlags.Browse, "browse", false,
		"open each proposed pull request in the browser")
	gitProposeCmd.Flags().BoolVar(&gitProposeFlags.Copy, "copy", false,
		"copy the proposed pull request URL(s) to the clipboard")
	gitCmd.AddCommand(gitProposeCmd)
}
