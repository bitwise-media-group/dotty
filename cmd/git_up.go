// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
)

var gitUpCmd = &cobra.Command{
	Use:   "up [num]",
	Short: "Move toward the tip of the current stack.",
	Long: `Checks out the layer [num] hops toward the stack tip (default 1) and
prints the stack.`,
	Example: `  dotty git up
  dotty git up 2`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		num, err := stackHops(args)
		if err != nil {
			return err
		}
		ios := cli.System()
		r := newRunner(ios)
		if _, err := git.Up(cmd.Context(), r, num); err != nil {
			return err
		}
		return printStackStatus(cmd.Context(), ios, r)
	},
}

func init() {
	gitCmd.AddCommand(gitUpCmd)
}
