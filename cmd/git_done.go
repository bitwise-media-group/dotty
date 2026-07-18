// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var gitDoneCmd = &cobra.Command{
	Use:   "done",
	Short: "Return to trunk, prune merged branches everywhere, fast-forward.",
	Long: `Finish a piece of work and reset to a clean, current trunk:

  1. Check out the trunk branch (main)
  2. Fetch all remotes with --prune
  3. Delete every local branch already merged into trunk (upstream/main when
     present, else origin/main), dropping it from any recorded stack
  4. Delete every origin branch already merged into trunk
  5. Fast-forward the local trunk branch to the remote

Any remaining stack that has diverged from trunk is reported; check it out
and run ` + "`dotty git sync`" + ` to rebase and re-sign it.`,
	Example: `  dotty git done`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ios := cli.System()
		r := newRunner(ios)
		ctx := cmd.Context()

		trunk, err := git.ResolveTrunk(ctx, r)
		if err != nil {
			return err
		}
		if err := git.Checkout(ctx, r, trunk.Branch); err != nil {
			return err
		}
		if err := git.FetchAllPrune(ctx, r); err != nil {
			tui.Warnf(ios, "Fetch: %v", err)
		}

		locals, err := git.MergedLocalBranches(ctx, r, trunk)
		if err != nil {
			return err
		}
		for _, b := range locals {
			if s, err := git.LoadStack(ctx, r, b); err == nil {
				if _, err := git.RemoveLayer(ctx, r, s, b); err != nil {
					tui.Warnf(ios, "Drop %s from stack %s: %v", b, s.ID, err)
				}
			} else if !errors.Is(err, git.ErrNotInStack) {
				tui.Warnf(ios, "%v", err)
			}
			if err := git.DeleteBranchLocal(ctx, r, b); err != nil {
				tui.Warnf(ios, "%v", err)
				continue
			}
			tui.Infof(ios, "Deleted %s", b)
		}

		if remote, err := git.PushRemote(ctx, r); err == nil {
			remotes, err := git.MergedRemoteBranches(ctx, r, remote, trunk)
			if err != nil {
				return err
			}
			for _, b := range remotes {
				if err := git.DeleteBranchRemote(ctx, r, remote, b); err != nil {
					tui.Warnf(ios, "%v", err)
					continue
				}
				tui.Infof(ios, "Deleted %s/%s", remote, b)
			}
		}

		if err := git.FastForward(ctx, r, trunk.Ref()); err != nil {
			return err
		}

		stacks, err := git.ListStacks(ctx, r)
		if err != nil {
			tui.Warnf(ios, "%v", err)
		}
		for _, s := range stacks {
			rows := git.Status(ctx, r, s, trunk, "")
			if git.AnyDiverged(rows) || git.AnyStale(ctx, r, rows) {
				tui.Warnf(ios, "Stack %s (%s) needs a rebase; check it out and run dotty git sync",
					s.ID, s.Bottom())
			}
		}

		tui.Successf(ios, "On %s, up to date with %s", trunk.Branch, trunk.Ref())
		return nil
	},
}

func init() {
	gitCmd.AddCommand(gitDoneCmd)
}
