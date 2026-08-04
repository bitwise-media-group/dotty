// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var privateRekeyCmd = &cobra.Command{
	Use:   "rekey",
	Short: "Re-encrypt every entry to the profile's current recipients.",
	Long: `Decrypt and re-encrypt all of the profile's ciphertext against the current
recipients file. Run it after enrolling a new security key — existing files
never learn about a recipient on their own — or after removing one, so the
departed key stops opening future revisions (it can still open anything
already in the git history; rotate the content itself when that matters).
Needs one currently-enrolled key plugged in.`,
	Example: `  dotty private rekey
  dotty private rekey --profile work`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		repo, err := resolvePrivateRepo()
		if err != nil {
			return err
		}
		name, err := privateProfileName()
		if err != nil {
			return err
		}
		identities, err := privdot.Identities(repo, name)
		if err != nil {
			return err
		}
		if err := privdot.RequireTerminal(ios, identities); err != nil {
			return err
		}
		dataDir, err := cli.DataDir()
		if err != nil {
			return err
		}
		count, err := privdot.Rekey(cmd.Context(), newRunner(ios), repo, name, dataDir)
		if err != nil {
			return err
		}
		tui.Successf(ios, "Re-encrypted %d files in profile %s; commit the result in %s", count, name, repo)
		return nil
	},
}

func init() {
	privateCmd.AddCommand(privateRekeyCmd)
}
