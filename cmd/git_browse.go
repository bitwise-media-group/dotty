// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
)

var gitBrowseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Open the upstream (else origin) repository page in a browser.",
	Long: `Opens the forge homepage for the upstream remote when present, otherwise
origin.`,
	Example: `  dotty git browse`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ios := cli.System()
		u, err := git.Browse(cmd.Context(), newRunner(ios))
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(ios.Out, u)
		return nil
	},
}

func init() {
	gitCmd.AddCommand(gitBrowseCmd)
}
