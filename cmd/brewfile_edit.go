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
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/brewfile"
	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// BrewfileEditFlags holds the flags for `dotty brewfile edit`.
type BrewfileEditFlags struct {
	Sync    bool
	Upgrade bool
}

var brewfileEditFlags = BrewfileEditFlags{}

var brewfileEditCmd = &cobra.Command{
	Use:   "edit [--sync | --upgrade]",
	Short: "Open the Brewfile in the default editor.",
	Long: `Open the Brewfile in $VISUAL / $EDITOR. With --sync or --upgrade, the
corresponding command runs after the editor exits.`,
	Example: `  dotty brewfile edit
  dotty brewfile edit --sync`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		path, err := resolveBrewfilePath()
		if err != nil {
			return err
		}
		runner := newRunner(ios)
		if err := cli.EditFile(cmd.Context(), runner, path); err != nil {
			return err
		}
		switch {
		case brewfileEditFlags.Sync:
			return brewfileSyncCmd.RunE(cmd, nil)
		case brewfileEditFlags.Upgrade:
			if err := brewfile.Upgrade(cmd.Context(), runner, path); err != nil {
				return err
			}
			tui.Successf(ios, "Upgraded brews from %s", path)
		}
		return nil
	},
}

func init() {
	brewfileEditCmd.Flags().BoolVar(&brewfileEditFlags.Sync, "sync", false, "run `dotty brewfile sync` after editing")
	brewfileEditCmd.Flags().BoolVar(&brewfileEditFlags.Upgrade, "upgrade", false, "run `dotty brewfile upgrade` after editing")
	brewfileEditCmd.MarkFlagsMutuallyExclusive("sync", "upgrade")
	brewfileCmd.AddCommand(brewfileEditCmd)
}
