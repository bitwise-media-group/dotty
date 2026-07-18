// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var gitSwitchCmd = &cobra.Command{
	Use:     "switch",
	Aliases: []string{"stack"},
	Short:   "Pick a stack layer and check it out.",
	Long: `Presents the current stack's layers in a fuzzy picklist and checks
out the chosen one.`,
	Example: `  dotty git switch
  dotty git stack     # alias`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ios := cli.System()
		r := newRunner(ios)
		ctx := cmd.Context()
		s, err := git.LoadStackForHEAD(ctx, r)
		if err != nil {
			return err
		}
		cur, err := git.CurrentBranch(ctx, r)
		if err != nil {
			return err
		}
		opts := make([]tui.Option, 0, len(s.Layers))
		for i, l := range s.Layers {
			label := fmt.Sprintf("[%d] %s", i+1, l.Branch)
			if l.TitleHint != "" {
				label += " — " + l.TitleHint
			}
			opts = append(opts, tui.Option{
				Label:    label,
				Value:    l.Branch,
				Selected: l.Branch == cur,
			})
		}
		picked, err := tui.FuzzySelect(ios, "Stack layer", opts)
		if err != nil {
			return err
		}
		if picked != cur {
			if err := git.Checkout(ctx, r, picked); err != nil {
				return err
			}
		}
		return printStackStatus(ctx, ios, r)
	},
}

func init() {
	gitCmd.AddCommand(gitSwitchCmd)
}
