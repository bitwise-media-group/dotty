// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var gitMergeFlags struct {
	All bool
	Up  int
	Yes bool
}

var gitMergeCmd = &cobra.Command{
	Use:   "merge [--all | --up=N]",
	Short: "Merge the current stack layer with its parent layer(s).",
	Long: `Collapse parent layers into the current branch. A stacked child already
carries its parents' commits, so merging deletes the absorbed parent
branches (local and origin, honouring dotty.stack.cleanup) and removes them
from the stack — the current branch's history does not change.

By default the immediate parent is merged. --up=N absorbs the N layers below
the current one, and --all absorbs everything down to the bottom of the
stack. Asking for more parents than the stack has below the current layer is
an error.

The merge range must be in sync first: every absorbed parent's commits must
already be contained in the current branch. When they are not — a parent was
amended, or trunk moved under the bottom layer — merge offers to rebase and
re-sign the layers from the bottom of the stack up to the current one, then
proceeds. An open PR on an absorbed parent closes when its branch is
deleted; the current layer's PR carries the work.`,
	Example: `  dotty git merge
  dotty git merge --up=2
  dotty git merge --all`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if gitMergeFlags.All && cmd.Flags().Changed("up") {
			return errors.New("--all and --up are mutually exclusive")
		}
		if cmd.Flags().Changed("up") && gitMergeFlags.Up < 1 {
			return errors.New("--up must be a positive integer")
		}
		ios := cli.System()
		r := newRunner(ios)
		return runGitMerge(cmd.Context(), ios, r)
	},
}

func init() {
	gitMergeCmd.Flags().BoolVar(&gitMergeFlags.All, "all", false,
		"merge every layer between the current one and the bottom of the stack")
	gitMergeCmd.Flags().IntVar(&gitMergeFlags.Up, "up", 1,
		"merge the current layer with this many parents")
	gitMergeCmd.Flags().BoolVarP(&gitMergeFlags.Yes, "yes", "y", false,
		"rebase+resign an out-of-sync merge range without prompting")
	gitCmd.AddCommand(gitMergeCmd)
}

// runGitMerge is the whole merge flow: resolve the range, get the stack
// below the current layer in sync (rebasing and re-signing when needed),
// absorb the parents, push what was rewritten, and refresh the PR maps.
func runGitMerge(ctx context.Context, ios cli.IOStreams, r *cli.ExecRunner) error {
	s, err := git.LoadStackForHEAD(ctx, r)
	if err != nil {
		return err
	}
	cur, err := git.CurrentBranch(ctx, r)
	if err != nil {
		return err
	}
	trunk, err := git.ResolveTrunk(ctx, r)
	if err != nil {
		return err
	}
	if err := git.FetchTrunk(ctx, r, trunk); err != nil {
		tui.Warnf(ios, "Fetch trunk: %v", err)
	}

	i := s.IndexOf(cur)
	if i < 0 {
		return fmt.Errorf("current branch %q is %w", cur, git.ErrNotInStack)
	}
	n := gitMergeFlags.Up
	if gitMergeFlags.All {
		n = i
	}
	if i == 0 {
		return fmt.Errorf("%s is the bottom of the stack; there is no parent to merge", cur)
	}
	if n > i {
		return fmt.Errorf("--up=%d exceeds the %d parent layer(s) below %s", n, i, cur)
	}

	rebased, err := syncMergeRange(ctx, ios, r, s, trunk, i)
	if err != nil {
		return err
	}

	for _, l := range s.Layers[i-n : i] {
		if l.PR > 0 {
			tui.Warnf(ios, "PR#%d (%s) will close when its branch is deleted", l.PR, l.Branch)
		}
	}
	s, merged, err := git.MergeParents(ctx, r, s, cur, n, git.DefaultCleanup(ctx, r))
	if err != nil {
		return err
	}
	tui.Successf(ios, "Merged %d layer(s) into %s: %s", len(merged), cur, strings.Join(merged, ", "))

	if err := git.Checkout(ctx, r, cur); err != nil {
		return err
	}
	if rebased {
		tui.Infof(ios, "Pushing %s", cur)
		if err := git.ForcePushBranch(ctx, r, cur); err != nil {
			return err
		}
		if i < len(s.Layers)+len(merged)-1 {
			tui.Warnf(ios, "Layers above %s were not rebased; run dotty git sync", cur)
		}
	}
	return refreshOpenPRBodies(ctx, ios, r, s, trunk)
}

// syncMergeRange gets layers bottom..upto stacked cleanly on one another (and
// the bottom on trunk): the first layer missing commits from its parent
// starts a rebase+resign chain up to upto. It reports whether anything was
// rewritten. Layers rewritten below the current one are about to be absorbed,
// so only the current branch needs pushing afterwards.
func syncMergeRange(ctx context.Context, ios cli.IOStreams, r *cli.ExecRunner,
	s git.Stack, trunk git.Trunk, upto int) (bool, error) {
	first := -1
	for j := 0; j <= upto; j++ {
		parent := git.ParentRevForLayer(s, j, trunk)
		missing, err := git.CommitsNotIn(ctx, r, parent, s.Layers[j].Branch)
		if err != nil {
			return false, err
		}
		if missing > 0 {
			first = j
			break
		}
	}
	if first < 0 {
		return false, nil
	}

	ok := gitMergeFlags.Yes
	if !ok {
		var cerr error
		ok, cerr = tui.ConfirmDefault(ios,
			fmt.Sprintf("The stack below %s is out of sync. Rebase + resign layers %s..%s?",
				s.Layers[upto].Branch, s.Layers[first].Branch, s.Layers[upto].Branch),
			"Rewrites SHAs; each layer will be re-signed with your hardware key.", true)
		if cerr != nil {
			if errors.Is(cerr, tui.ErrNotInteractive) {
				return false, fmt.Errorf("%w (pass --yes to rebase non-interactively)", cerr)
			}
			return false, cerr
		}
	}
	if !ok {
		return false, errors.New("merge range out of sync; re-run with confirmation to rebase+resign")
	}

	for k := first; k <= upto; k++ {
		base := git.ParentRevForLayer(s, k, trunk)
		branch := s.Layers[k].Branch
		tui.Infof(ios, "Rebasing %s onto %s", branch, base)
		if err := git.RebaseOnto(ctx, r, branch, base); err != nil {
			return false, fmt.Errorf(
				"%w\nresolve the conflicts (git rebase --continue), then re-run dotty git merge", err)
		}
		tui.Infof(ios, "Re-signing %s", branch)
		if err := resignRange(ctx, r, base); err != nil {
			return false, err
		}
	}
	return true, nil
}
