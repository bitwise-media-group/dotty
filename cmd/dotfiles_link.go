// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/linker"
)

// DotfilesLinkFlags holds the flags for `dotty dotfiles link`.
type DotfilesLinkFlags struct {
	OnConflict string
}

var dotfilesLinkFlags = DotfilesLinkFlags{}

var dotfilesLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Symlink the repository's home tree into your home directory.",
	Long: `Link the repository's home/ tree over $HOME: whole files and directories
are linked folded, existing real directories are descended into, and stale
symlinks are replaced. A real file in the way is resolved per --on-conflict;
link defaults to fail so an unexpected file stops a re-link instead of being
moved. Legacy files that shadow the rendered configuration from outside any
link site (~/.gitconfig, ~/.zshrc and the other bare zsh startup files) are
backed up and removed; restore them with dotty dotfiles restore.`,
	Example: `  dotty dotfiles link
  dotty dotfiles link --repo ~/Repos/dotfiles --on-conflict=backup`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		repo, answers, err := resolveDotfilesRepo()
		if err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		configDir, err := cli.ConfigDir()
		if err != nil {
			return err
		}
		if err := linker.MigrateCodexConfig(ios, home, configDir); err != nil {
			return err
		}
		report, backupDir, err := linker.LinkHome(ios, answers, repo, home, dotfilesLinkFlags.OnConflict)
		if err != nil {
			return err
		}
		linker.Summarize(ios, report, backupDir)
		return nil
	},
}

func init() {
	dotfilesLinkCmd.Flags().StringVar(&dotfilesLinkFlags.OnConflict, "on-conflict", "fail",
		"existing-file resolution: backup, adopt, skip, or fail")
	dotfilesCmd.AddCommand(dotfilesLinkCmd)
}
