// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/mise"
)

// PackagesEditFlags holds the flags for `dotty packages edit`.
type PackagesEditFlags struct {
	Sync    bool
	Upgrade bool
}

var packagesEditFlags = PackagesEditFlags{}

var packagesEditCmd = &cobra.Command{
	Use:   "edit [--sync | --upgrade]",
	Short: "Open the profile's config.toml in the default editor.",
	Long: `Open the profile's user-owned mise config.toml in $VISUAL / $EDITOR. The
component fragments under conf.d/ are re-rendered by dotty init and are not
the place for edits. With --sync or --upgrade, the corresponding command
runs after the editor exits.`,
	Example: `  dotty packages edit
  dotty packages edit --sync`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		dir, err := resolvePackagesDir()
		if err != nil {
			return err
		}
		if err := cli.EditFile(cmd.Context(), newRunner(ios), mise.ConfigPath(dir)); err != nil {
			return err
		}
		switch {
		case packagesEditFlags.Sync:
			return packagesSyncCmd.RunE(cmd, nil)
		case packagesEditFlags.Upgrade:
			return packagesUpgradeCmd.RunE(cmd, nil)
		}
		return nil
	},
}

func init() {
	packagesEditCmd.Flags().BoolVar(&packagesEditFlags.Sync, "sync", false, "run `dotty packages sync` after editing")
	packagesEditCmd.Flags().BoolVar(&packagesEditFlags.Upgrade, "upgrade", false,
		"run `dotty packages upgrade` after editing")
	packagesEditCmd.MarkFlagsMutuallyExclusive("sync", "upgrade")
	packagesCmd.AddCommand(packagesEditCmd)
}
