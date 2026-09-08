// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

var packagesStatusCmd = &cobra.Command{
	Use:     "status",
	Aliases: []string{"ls"},
	Short:   "List the profile's packages and whether they are installed.",
	Long: `Print the profile's tools with their installed versions (mise ls --global)
and its bootstrap packages with their state (mise bootstrap packages
status).`,
	Example: `  dotty packages status
  dotty packages ls`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newMiseClient(cmd.Context(), cli.System())
		if err != nil {
			return err
		}
		return client.Status(cmd.Context())
	},
}

func init() {
	packagesCmd.AddCommand(packagesStatusCmd)
}
