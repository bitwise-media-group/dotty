// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/brewfile"
	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// BrewfileSyncFlags holds the flags for `dotty brewfile sync`.
type BrewfileSyncFlags struct {
	Force bool
}

var brewfileSyncFlags = BrewfileSyncFlags{}

var brewfileSyncCmd = &cobra.Command{
	Use:   "sync [--force]",
	Short: "Make the machine match the Brewfile exactly.",
	Long: `Synchronise the machine with the Brewfile — install what's listed, upgrade
what's outdated, and remove (zap) what isn't listed. When anything would be
removed, dotty shows the list and asks first unless --force is set.`,
	Example: `  dotty brewfile sync
  dotty brewfile sync --force`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		path, err := resolveBrewfilePath()
		if err != nil {
			return err
		}
		aborted := false
		confirm := func(removals []string) (bool, error) {
			tui.Warnf(ios, "Syncing will remove brews not in the Brewfile:")
			for _, line := range removals {
				fmt.Fprintf(ios.ErrOut, "    %s\n", line)
			}
			ok, err := tui.Confirm(ios, "Remove them and continue?", "")
			if errors.Is(err, tui.ErrNotInteractive) {
				return false, errors.New("sync would remove brews; re-run interactively or pass --force")
			}
			if errors.Is(err, tui.ErrAborted) {
				err = nil
			}
			aborted = !ok
			return ok, err
		}
		if err := brewfile.Sync(cmd.Context(), newRunner(ios), path, brewfileSyncFlags.Force, confirm); err != nil {
			return err
		}
		if aborted {
			tui.Infof(ios, "Sync aborted; nothing changed")
			return nil
		}
		tui.Successf(ios, "Machine synced with %s", path)
		return nil
	},
}

func init() {
	brewfileSyncCmd.Flags().BoolVar(&brewfileSyncFlags.Force, "force", false,
		"remove unlisted brews without asking")
	brewfileCmd.AddCommand(brewfileSyncCmd)
}
