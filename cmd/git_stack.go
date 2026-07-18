// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// gitStackCmd is stack status (not "git status"): lineage and relation to trunk.
var gitStackCmd = &cobra.Command{
	Use:   "stack",
	Short: "Show the current stack versus trunk.",
	Long: `Print the current branch's stack: each layer, its relation to trunk
(ff, merged, diverged, identical), and any linked PR numbers.

If this branch is not yet recorded in git config but local branches form an
obvious linear lineage of at least three nodes (trunk plus two or more feature
branches), the lineage is detected and saved automatically. A single branch off
trunk is not treated as a stack.

This is not git status — it is the status of the stacked branch chain managed
by start / append / propose / sync.`,
	Example: `  dotty git stack`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ios := cli.System()
		return printStackStatus(cmd.Context(), ios, newRunner(ios))
	},
}

func init() {
	gitCmd.AddCommand(gitStackCmd)
}

// printStackStatus writes the current stack versus trunk to ios.Out.
// Used after up/down/switch and by `dotty git stack`.
func printStackStatus(ctx context.Context, ios cli.IOStreams, r *cli.ExecRunner) error {
	trunk, err := git.ResolveTrunk(ctx, r)
	if err != nil {
		return err
	}
	_ = git.FetchTrunk(ctx, r, trunk)
	s, discovered, err := git.LoadOrDiscoverStack(ctx, r, trunk)
	if err != nil {
		return err
	}
	if discovered {
		tui.Infof(ios, "Discovered stack %s from local branch lineage", s.ID)
	}
	cur, err := git.CurrentBranch(ctx, r)
	if err != nil {
		return err
	}
	rows := git.Status(ctx, r, s, trunk, cur)
	git.FormatStatus(ios.Out, s, trunk, rows)
	return nil
}
