// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var gitStartCmd = &cobra.Command{
	Use:   "start <branch>",
	Short: "Create a branch from trunk and start a new stack.",
	Long: `Creates <branch> from the trunk (upstream/main when present, else
origin/main), records it as the first layer of a new stack, and pushes it
to the push remote with upstream tracking set.`,
	Example: `  dotty git start feat-1`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		r := newRunner(ios)
		ctx := cmd.Context()
		trunk, err := git.ResolveTrunk(ctx, r)
		if err != nil {
			return err
		}
		s, err := git.Start(ctx, r, trunk, args[0])
		if err != nil {
			return err
		}
		if err := git.PublishBranch(ctx, r, args[0]); err != nil {
			tui.Warnf(ios, "%v — run `git push` once the remote is reachable", err)
		}
		tui.Successf(ios, "Started stack %s on %s (from %s)", s.ID, args[0], trunk.Ref())
		return nil
	},
}

func init() {
	gitCmd.AddCommand(gitStartCmd)
}
