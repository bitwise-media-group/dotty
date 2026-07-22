// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/brewfile"
	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// BrewfileAddFlags holds one bool per brew bundle entry type; exactly one (or
// none, meaning formula) may be set.
type BrewfileAddFlags struct {
	Formula bool
	Cask    bool
	Tap     bool
	VSCode  bool
	Go      bool
	Cargo   bool
	UV      bool
	Flatpak bool
	Krew    bool
	NPM     bool
}

var brewfileAddFlags = BrewfileAddFlags{}

var brewfileAddCmd = &cobra.Command{
	Use:   "add [--tap | --cask | --formula | ...] <name> [...]",
	Short: "Add brews to the Brewfile and install them.",
	Long: `Add one or more entries to the Brewfile, then install the bundle. Entries
default to formulae; pass a type flag for anything else. Entries the Brewfile
already lists (per brew's own parser) are skipped rather than duplicated —
the bundle is still installed. Tap-qualified names (more than one slash) of
formulae and casks, and taps themselves, go through Homebrew's trust gate
first: dotty asks before trusting anything new and records "trusted: true" on
the new Brewfile entry, so the trust survives the trust-store reset that
` + "`dotty brewfile sync`" + ` performs. Taps that tap-qualified names refer
to are tapped first when missing — Homebrew no longer installs a tap
implicitly when a formula or cask is named through it.`,
	Example: `  dotty brewfile add ripgrep jq
  dotty brewfile add --cask ghostty
  dotty brewfile add --tap fluxcd/tap
  dotty brewfile add acme/tap/widget`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		path, err := resolveBrewfilePath()
		if err != nil {
			return err
		}
		kind := brewfileAddFlags.kind()
		confirmTrust := func(name string) (bool, error) {
			ok, err := tui.Confirm(ios,
				fmt.Sprintf("Trust %s %q?", kind, name),
				"It comes from a third-party tap and is not yet in Homebrew's trust store.")
			if errors.Is(err, tui.ErrNotInteractive) {
				return false, fmt.Errorf("%s %q needs trust; run interactively or `brew trust --%s %s` first",
					kind, name, kind, name)
			}
			if errors.Is(err, tui.ErrAborted) {
				return false, nil
			}
			return ok, err
		}
		res, err := brewfile.Add(cmd.Context(), newRunner(ios), path, kind, args, confirmTrust)
		if err != nil {
			return err
		}
		for _, name := range res.Skipped {
			tui.Infof(ios, "%s %q is already in the Brewfile; skipped", kind, name)
		}
		if len(res.Unmarked) > 0 {
			tui.Warnf(ios, "could not mark %s as trusted in %s; add `, trusted: true` to the entr%s by hand "+
				"or `dotty brewfile sync` will revoke the trust",
				strings.Join(res.Unmarked, ", "), path, plural(len(res.Unmarked), "y", "ies"))
		}
		added := len(args) - len(res.Skipped)
		if added == 0 {
			tui.Successf(ios, "Brewfile already lists all %d entr%s; installed the bundle at %s",
				len(args), plural(len(args), "y", "ies"), path)
		} else {
			tui.Successf(ios, "Added %d %s entr%s to %s", added, kind, plural(added, "y", "ies"), path)
		}
		return nil
	},
}

// kind maps the set flag to its brewfile kind; formulae are the default.
func (f BrewfileAddFlags) kind() brewfile.Kind {
	switch {
	case f.Cask:
		return brewfile.KindCask
	case f.Tap:
		return brewfile.KindTap
	case f.VSCode:
		return brewfile.KindVSCode
	case f.Go:
		return brewfile.KindGo
	case f.Cargo:
		return brewfile.KindCargo
	case f.UV:
		return brewfile.KindUV
	case f.Flatpak:
		return brewfile.KindFlatpak
	case f.Krew:
		return brewfile.KindKrew
	case f.NPM:
		return brewfile.KindNPM
	default:
		return brewfile.KindFormula
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func init() {
	f := brewfileAddCmd.Flags()
	f.BoolVar(&brewfileAddFlags.Formula, "formula", false, "add formulae (the default)")
	f.BoolVar(&brewfileAddFlags.Cask, "cask", false, "add casks")
	f.BoolVar(&brewfileAddFlags.Tap, "tap", false, "add taps")
	f.BoolVar(&brewfileAddFlags.VSCode, "vscode", false, "add VSCode extensions")
	f.BoolVar(&brewfileAddFlags.Go, "go", false, "add Go packages")
	f.BoolVar(&brewfileAddFlags.Cargo, "cargo", false, "add Cargo packages")
	f.BoolVar(&brewfileAddFlags.UV, "uv", false, "add uv tools")
	f.BoolVar(&brewfileAddFlags.Flatpak, "flatpak", false, "add Flatpak packages")
	f.BoolVar(&brewfileAddFlags.Krew, "krew", false, "add Krew plugins")
	f.BoolVar(&brewfileAddFlags.NPM, "npm", false, "add npm packages")
	brewfileAddCmd.MarkFlagsMutuallyExclusive(
		"formula", "cask", "tap", "vscode", "go", "cargo", "uv", "flatpak", "krew", "npm")
	brewfileCmd.AddCommand(brewfileAddCmd)
}
