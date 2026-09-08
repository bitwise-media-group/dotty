// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/mise"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// PackagesImportFlags holds the flags for `dotty packages import`.
type PackagesImportFlags struct {
	All      bool
	Brewfile string
}

var packagesImportFlags = PackagesImportFlags{}

var packagesImportCmd = &cobra.Command{
	Use:   "import [--all] [--brewfile <path>]",
	Short: "Import installed Homebrew formulae, or a Brewfile, into the profile.",
	Long: `Record what is already on the machine in the profile's config.toml. By
default the Homebrew formulae installed on request become brew: bootstrap
packages (with the taps they need); --all includes every linked formula,
dependencies too. With --brewfile the given Brewfile is converted instead:
formulae the components already provide as tools are dropped, other formulae
become brew: packages, casks become brew-cask: packages restricted to
macOS, and entry types mise has no equivalent for are reported. Either way
entries the profile already declares are skipped, and new ones land under a
header comment so the import is idempotent.`,
	Example: `  dotty packages import
  dotty packages import --all
  dotty packages import --brewfile ~/Brewfile`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		if packagesImportFlags.Brewfile != "" {
			dir, err := resolvePackagesDir()
			if err != nil {
				return err
			}
			path, err := cli.ExpandHome(packagesImportFlags.Brewfile)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read Brewfile: %w", err)
			}
			conv := mise.ConvertBrewfile(data)
			for _, w := range conv.Warnings {
				tui.Warnf(ios, "%s", w)
			}
			added, err := mise.MergeConversion(dir, conv)
			if err != nil {
				return err
			}
			tui.Successf(ios, "Converted %d entr%s from %s into %s", added, plural(added, "y", "ies"),
				path, mise.ConfigPath(dir))
			return nil
		}

		client, err := newMiseClient(cmd.Context(), ios)
		if err != nil {
			return err
		}
		entries, err := mise.ImportToScratch(cmd.Context(), newRunner(ios), client.Bin, client.Dir,
			packagesImportFlags.All)
		if err != nil {
			return err
		}
		added, err := mise.MergeImported(client.Dir, entries)
		if err != nil {
			return err
		}
		tui.Successf(ios, "Imported %d entr%s into %s", added, plural(added, "y", "ies"), mise.ConfigPath(client.Dir))
		return nil
	},
}

func init() {
	packagesImportCmd.Flags().BoolVar(&packagesImportFlags.All, "all", false,
		"import every linked formula, dependencies included")
	packagesImportCmd.Flags().StringVar(&packagesImportFlags.Brewfile, "brewfile", "",
		"convert this Brewfile instead of the installed formulae")
	packagesImportCmd.MarkFlagsMutuallyExclusive("all", "brewfile")
	packagesCmd.AddCommand(packagesImportCmd)
}
