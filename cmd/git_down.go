// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
)

var gitDownCmd = &cobra.Command{
	Use:   "down [num]",
	Short: "Move toward the trunk of the current stack.",
	Long: `Checks out the layer [num] hops toward the trunk (default 1) and
prints the stack.`,
	Example: `  dotty git down
  dotty git down 2`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		num, err := stackHops(args)
		if err != nil {
			return err
		}
		ios := cli.System()
		r := newRunner(ios)
		if _, err := git.Down(cmd.Context(), r, num); err != nil {
			return err
		}
		return printStackStatus(cmd.Context(), ios, r)
	},
}

func init() {
	gitCmd.AddCommand(gitDownCmd)
}
