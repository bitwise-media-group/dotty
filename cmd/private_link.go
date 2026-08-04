// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/linker"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// PrivateLinkFlags holds the flags for `dotty private link`.
type PrivateLinkFlags struct {
	OnConflict string
	Strict     bool
}

var privateLinkFlags = PrivateLinkFlags{}

var privateLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Decrypt the profile's private tree and link it into $HOME.",
	Long: `Materialize the active profile's plaintext under the dotty data directory
(0700) and symlink it over $HOME through the private active-profile
indirection, so activating another profile swaps every private file at once.
Decryption is incremental — only entries whose ciphertext changed are
decrypted, so a steady-state relink never touches the security key. Entries
that cannot be decrypted (key unplugged) or that carry unreconciled local
edits are skipped with a warning and the previous copy stays usable;
--strict turns any skip into a failure. Existing real files at link sites
resolve per --on-conflict; by default dotty asks (backing up when there is
no terminal).`,
	Example: `  dotty private link
  dotty private link --on-conflict=backup --strict`,
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
		return runPrivateLink(cmd.Context(), ios, repo, name,
			privateLinkFlags.OnConflict, privateLinkFlags.Strict)
	},
}

// runPrivateLink is the whole link flow — materialize, activate, link,
// prune — shared with the init and profile-activate integrations.
func runPrivateLink(ctx context.Context, ios cli.IOStreams, repo, name, onConflict string, strict bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	dataDir, err := cli.DataDir()
	if err != nil {
		return err
	}
	rep, err := privdot.Materialize(ctx, ios, newRunner(ios), repo, name, dataDir, strict)
	if err != nil {
		return err
	}
	if err := privdot.Activate(dataDir, name); err != nil {
		return err
	}
	report, backupDir, err := linker.LinkPrivate(ios, dataDir, home, onConflict)
	if err != nil {
		return err
	}
	linker.Summarize(ios, report, backupDir)

	pruned := rep.Pruned
	if stale, err := privdot.StaleRels(repo, name); err == nil {
		pruned = append(pruned, stale...)
	}
	linker.PruneSites(ios, home, pruned)

	tui.Successf(ios, "Private profile %s: %d decrypted, %d copied, %d already current",
		name, rep.Decrypted, rep.Copied, rep.UpToDate)
	if len(rep.Skipped) > 0 {
		tui.Warnf(ios, "Skipped %d entries: %s", len(rep.Skipped), strings.Join(rep.Skipped, ", "))
	}
	return nil
}

func init() {
	privateLinkCmd.Flags().StringVar(&privateLinkFlags.OnConflict, "on-conflict", "",
		"existing-file resolution: backup, adopt, skip, or fail (default: ask; backup when not a terminal)")
	privateLinkCmd.Flags().BoolVar(&privateLinkFlags.Strict, "strict", false,
		"fail on any entry that cannot be decrypted or conflicts")
	privateCmd.AddCommand(privateLinkCmd)
}
