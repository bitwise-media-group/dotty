// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var gitAppendCmd = &cobra.Command{
	Use:   "append <branch>",
	Short: "Create a child branch on the stack tip.",
	Long: `Creates <branch> at the current stack tip and records it as a new
layer in the stack lineage.`,
	Example: `  dotty git append feat-2`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		r := newRunner(ios)
		s, err := git.Append(cmd.Context(), r, args[0])
		if err != nil {
			return err
		}
		tui.Successf(ios, "Appended %s to stack %s (%d layers)", args[0], s.ID, len(s.Layers))
		return nil
	},
}

func init() {
	gitCmd.AddCommand(gitAppendCmd)
}
