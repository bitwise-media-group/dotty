// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var packagesUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade every package the profile declares.",
	Long: `Move every tool and bootstrap package to its newest version without removing
anything — mise upgrade, then mise bootstrap packages upgrade — and refresh
mise.lock for every platform so the new versions travel with the profile.`,
	Example: `  dotty packages upgrade
  dotty --profile=work packages upgrade`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		client, err := newMiseClient(cmd.Context(), ios)
		if err != nil {
			return err
		}
		if err := client.Upgrade(cmd.Context()); err != nil {
			return err
		}
		tui.Successf(ios, "Upgraded the packages of %s", client.Dir)
		return nil
	},
}

func init() {
	packagesCmd.AddCommand(packagesUpgradeCmd)
}
