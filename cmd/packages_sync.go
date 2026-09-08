// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// PackagesSyncFlags holds the flags for `dotty packages sync`.
type PackagesSyncFlags struct {
	Force bool
}

var packagesSyncFlags = PackagesSyncFlags{}

var packagesSyncCmd = &cobra.Command{
	Use:   "sync [--force]",
	Short: "Make the machine match the profile's packages exactly.",
	Long: `Synchronise the machine with the profile: refresh mise.lock for every
platform, install the locked tools and the bootstrap packages that are
missing, prune tool versions the profile no longer references, and remove
the bootstrap packages it no longer declares. When bootstrap packages would be
removed, dotty shows the list and asks first unless --force is set. Cask
removal is conservative: mise only removes casks it installed itself, so an
app installed by hand may stay put.`,
	Example: `  dotty packages sync
  dotty packages sync --force`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		client, err := newMiseClient(cmd.Context(), ios)
		if err != nil {
			return err
		}
		aborted := false
		confirm := func(removals []string) (bool, error) {
			tui.Warnf(ios, "Syncing will remove packages the profile no longer declares:")
			for _, line := range removals {
				_, _ = fmt.Fprintf(ios.ErrOut, "    %s\n", line)
			}
			ok, err := tui.Confirm(ios, "Remove them and continue?", "")
			if errors.Is(err, tui.ErrNotInteractive) {
				return false, errors.New("sync would remove packages; re-run interactively or pass --force")
			}
			if errors.Is(err, tui.ErrAborted) {
				err = nil
			}
			aborted = !ok
			return ok, err
		}
		if err := client.Sync(cmd.Context(), packagesSyncFlags.Force, confirm); err != nil {
			return err
		}
		if aborted {
			tui.Infof(ios, "Sync aborted; nothing changed")
			return nil
		}
		tui.Successf(ios, "Machine synced with %s", client.Dir)
		return nil
	},
}

func init() {
	packagesSyncCmd.Flags().BoolVar(&packagesSyncFlags.Force, "force", false,
		"remove undeclared packages without asking")
	packagesCmd.AddCommand(packagesSyncCmd)
}
