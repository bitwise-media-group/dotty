// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var packagesLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Refresh mise.lock for every platform.",
	Long: `Resolve every tool the profile declares for every platform it publishes and
write the versions, URLs, and checksums to the profile's mise.lock — the one
committed lockfile serves macOS and Linux. add, sync, and upgrade do this
on their own; lock is for after editing config.toml by hand.`,
	Example: `  dotty packages lock`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		client, err := newMiseClient(cmd.Context(), ios)
		if err != nil {
			return err
		}
		if err := client.Lock(cmd.Context()); err != nil {
			return err
		}
		tui.Successf(ios, "Locked the tools of %s", client.Dir)
		return nil
	},
}

func init() {
	packagesCmd.AddCommand(packagesLockCmd)
}
