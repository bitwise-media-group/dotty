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
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/brewfile"
	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// BrewfileDumpFlags holds the flags for `dotty brewfile dump`.
type BrewfileDumpFlags struct {
	All bool
}

var brewfileDumpFlags = BrewfileDumpFlags{}

var brewfileDumpCmd = &cobra.Command{
	Use:   "dump [--all]",
	Short: "Snapshot the installed brews into the Brewfile.",
	Long: `Write the currently installed brews into the Brewfile. By default only
formulae, casks, Mac App Store apps, and Flatpaks are dumped; --all includes
every type brew bundle knows (taps, vscode, go, cargo, uv, krew, npm).
Overwriting an existing Brewfile asks first.`,
	Example: `  dotty brewfile dump
  dotty brewfile dump --all`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		path, err := resolveBrewfilePath()
		if err != nil {
			return err
		}
		force := false
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			ok, err := tui.Confirm(ios, fmt.Sprintf("Overwrite the existing Brewfile at %s?", path), "")
			if errors.Is(err, tui.ErrNotInteractive) {
				return fmt.Errorf("%s already exists; remove it or run interactively to confirm overwriting", path)
			}
			if err != nil && !errors.Is(err, tui.ErrAborted) {
				return err
			}
			if !ok {
				tui.Infof(ios, "Dump aborted; nothing written")
				return nil
			}
			force = true
		}
		if err := brewfile.Dump(cmd.Context(), newRunner(ios), path, brewfileDumpFlags.All, force); err != nil {
			return err
		}
		tui.Successf(ios, "Wrote %s", path)
		return nil
	},
}

func init() {
	brewfileDumpCmd.Flags().BoolVar(&brewfileDumpFlags.All, "all", false,
		"dump every entry type brew bundle supports")
	brewfileCmd.AddCommand(brewfileDumpCmd)
}
