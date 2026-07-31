// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/linker"
	"github.com/bitwise-media-group/dotty/internal/scaffold"
)

var dotfilesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report the link state of the dotfiles tree without changing it.",
	Long: `Walk the repository's home/ tree against $HOME and report each entry: ok
(linked correctly), missing (link would be created), stale (a symlink
pointing elsewhere), or conflict (a real file in the way). Copy-deployed
directories (grok's hooks) report the same states for their mirrored files:
stale means the mirror is out of sync or still the retired symlink.`,
	Example: `  dotty dotfiles status`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		repo, _, err := resolveDotfilesRepo()
		if err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}

		actions, err := linker.Status(linker.Tree{Source: scaffold.HomeDir(repo), Target: home, Copy: scaffold.CopyDirs})
		if err != nil {
			return err
		}
		labels := map[linker.State]string{
			linker.StateOK: "ok", linker.StateLink: "missing",
			linker.StateRelink: "stale", linker.StateConflict: "conflict",
		}
		ok := 0
		for _, a := range actions {
			if a.State == linker.StateOK {
				ok++
				continue
			}
			_, _ = fmt.Fprintf(ios.Out, "%-8s  %s\n", labels[a.State], a.Site)
		}
		_, _ = fmt.Fprintf(ios.Out, "%d linked correctly, %d needing attention\n", ok, len(actions)-ok)
		return nil
	},
}

func init() {
	dotfilesCmd.AddCommand(dotfilesStatusCmd)
}
