// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var gitExpandFlags struct {
	AutoSquash bool
}

var gitExpandCmd = &cobra.Command{
	Use:   "expand [--auto-squash]",
	Short: "Expand the current branch into a stack with one layer per commit.",
	Long: `Turn the current branch's commits (trunk..HEAD) into a stack: every
commit gets its own layer branch, named from its subject, and the current
branch stays as the tip. Without --auto-squash no history is rewritten —
layer branches simply point at the existing commits.

With --auto-squash, each chore commit is first squashed into the commit
below it, so chores never form a layer of their own (a chore that is the
very first commit has nothing below it and keeps its own layer). Squashing
rewrites history from the first squash onward; rewritten commits are created
with plain git commits, so your commit signing configuration re-signs them.

The planned stack is always shown first and nothing changes until you
confirm. The branch is expanded in place — it is not rebased onto trunk; run
` + "`dotty git propose`" + ` or ` + "`dotty git sync`" + ` afterwards as usual.`,
	Example: `  dotty git expand
  dotty git expand --auto-squash`,
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

		// A branch already registered as a single-layer stack (e.g. adopted
		// by propose) expands in place, keeping its stack id and PR; a branch
		// inside a real stack cannot be expanded again.
		reuseID := ""
		tipPR := 0
		existing, err := git.LoadStack(ctx, r, cur)
		switch {
		case err == nil && len(existing.Layers) > 1:
			return fmt.Errorf("%s is already in a stack of %d layers; expand works on single branches",
				cur, len(existing.Layers))
		case err == nil:
			reuseID = existing.ID
			tipPR = existing.Layers[0].PR
		case !errors.Is(err, git.ErrNotInStack):
			return err
		}

		p, err := git.PlanExpand(ctx, r, trunk, cur, gitExpandFlags.AutoSquash)
		if err != nil {
			return err
		}
		if p.Squashes() {
			clean, err := git.IsWorktreeClean(ctx, r)
			if err != nil {
				return err
			}
			if !clean {
				return errors.New("--auto-squash rewrites commits; commit or stash your changes first")
			}
		}

		git.FormatExpandPlan(ios.Out, p, trunk)
		detail := "Creates the layer branches shown above; no commits are rewritten."
		if p.Squashes() {
			detail = "Rewrites this branch from the first squash onward (new SHAs, re-signed " +
				"via your git signing configuration), then creates the layer branches."
		}
		ok, err := tui.Confirm(ios,
			fmt.Sprintf("Expand %s into this %d-layer stack?", cur, len(p.Layers)), detail)
		if err != nil {
			if errors.Is(err, tui.ErrNotInteractive) {
				return fmt.Errorf("%w (expand always confirms before changing refs)", err)
			}
			return err
		}
		if !ok {
			tui.Infof(ios, "Aborted; no changes made")
			return nil
		}

		s, err := git.ExecuteExpand(ctx, r, p, reuseID)
		if err != nil {
			return err
		}
		if tipPR > 0 {
			s.Layers[len(s.Layers)-1].PR = tipPR
			if err := git.SaveStack(ctx, r, s); err != nil {
				return err
			}
		}
		tui.Successf(ios, "Expanded %s into stack %s (%d layers)", cur, s.ID, len(s.Layers))
		rows := git.Status(ctx, r, s, trunk, cur)
		git.FormatStatus(ios.Out, s, trunk, rows)
		return nil
	},
}

func init() {
	gitExpandCmd.Flags().BoolVar(&gitExpandFlags.AutoSquash, "auto-squash", false,
		"squash each chore commit into the commit below it before expanding")
	gitCmd.AddCommand(gitExpandCmd)
}
