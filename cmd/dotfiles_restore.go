// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/linker"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// DotfilesRestoreFlags holds the flags for `dotty dotfiles restore`.
type DotfilesRestoreFlags struct {
	Timestamp string
}

var dotfilesRestoreFlags = DotfilesRestoreFlags{}

var dotfilesRestoreCmd = &cobra.Command{
	Use:   "restore [path ...]",
	Short: "Restore mise history or an existing dotty backup set.",
	Long: `With paths or mise flags, proxy mise dot rollback to restore tracked files
from their checkpoints. Repository sources are tracked by link; restoring a
copied source requires dotty dotfiles apply to refresh its live copy.

With --timestamp, or with no arguments, restore the files a link run backed up:
every file in the chosen backup set
is copied back to the absolute path it came from, replacing the symlink that
displaced it. Without --timestamp, an interactive picklist offers the
available sets, newest first. The set is copied, not consumed — a restore can
be repeated.`,
	Example: `  dotty dotfiles restore
  dotty dotfiles restore ~/.zshrc --dry-run
  dotty dotfiles restore --timestamp=2026-07-15T10-30-00`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		own, rest, help := cli.ExtractFlags(args, map[string]bool{"timestamp": true, "repo": true})
		if repo, ok := own["repo"]; ok {
			dotfilesFlags.Repo = repo
		}
		if help {
			return cmd.Help()
		}
		if _, legacy := own["timestamp"]; !legacy && len(rest) > 0 {
			return proxyDotfiles(cmd, append([]string{"rollback"}, rest...))
		}
		if len(rest) > 0 {
			return fmt.Errorf("--timestamp cannot be combined with mise rollback arguments")
		}
		ios := cli.System()
		dataDir, err := cli.DataDir()
		if err != nil {
			return err
		}
		root := filepath.Join(dataDir, "backups")

		ts := own["timestamp"]
		if ts == "" {
			ts, err = pickBackupSet(ios, root)
			if errors.Is(err, tui.ErrAborted) || ts == "" && err == nil {
				return nil
			}
			if err != nil {
				return err
			}
		}

		restored, err := linker.Restore(filepath.Join(root, ts))
		if err != nil {
			return err
		}
		tui.Successf(ios, "Restored %d files from %s", len(restored), filepath.Join(root, ts))
		return nil
	},
}

func init() {
	dotfilesRestoreCmd.Flags().StringVar(&dotfilesRestoreFlags.Timestamp, "timestamp", "",
		"backup set to restore (a directory name under the backups dir)")
	dotfilesCmd.AddCommand(dotfilesRestoreCmd)
}

// pickBackupSet lists the backup sets newest-first and asks which to restore.
func pickBackupSet(ios cli.IOStreams, root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("read backups dir %s: %w", root, err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no backup sets under %s", root)
	}
	options := make([]tui.Option, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- { // lexicographic timestamps: newest last
		if entries[i].IsDir() {
			options = append(options, tui.Option{Label: entries[i].Name(), Value: entries[i].Name()})
		}
	}
	if len(options) == 0 {
		return "", fmt.Errorf("no backup sets under %s", root)
	}
	picked, err := tui.FuzzySelect(ios, "Restore which backup set?", options)
	if errors.Is(err, tui.ErrNotInteractive) {
		return "", errors.New("several backup sets exist; pass --timestamp")
	}
	return picked, err
}
