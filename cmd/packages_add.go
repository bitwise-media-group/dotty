// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/mise"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var packagesAddCmd = &cobra.Command{
	Use:   "add <id> [...]",
	Short: "Add packages to the profile and install them.",
	Long: `Record one or more packages in the profile's config.toml and install them.
Tools (aqua:owner/repo, github:owner/repo, npm:name, pipx:name, go:module,
cargo:crate, or a mise core tool like go or node) go through mise use and
are locked for every platform in mise.lock; bootstrap packages (brew:name,
brew-cask:name, mas:id, flatpak:id, apt:name, …) go through mise bootstrap
packages use and track their latest version. Ids the profile already
declares — in config.toml or a component fragment — are skipped rather than
duplicated; the profile is still installed. Bare names are refused: a
registry shorthand like git or flux resolves to whichever entry claims it
(git-chglog, flux-operator), so spell the backend.`,
	Example: `  dotty packages add aqua:sharkdp/bat aqua:jqlang/jq
  dotty packages add brew-cask:raycast
  dotty packages add github:bitwise-media-group/evolve
  dotty packages add npm:prettier`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		for _, id := range args {
			if !mise.IsPackageID(id) && !mise.IsToolID(id) {
				return fmt.Errorf("%q is not a full package id; use a backend id such as aqua:owner/%s or brew:%s"+
					" (`mise registry %s` lists the candidates)", id, id, id, id)
			}
		}
		client, err := newMiseClient(cmd.Context(), ios)
		if err != nil {
			return err
		}
		res, err := client.Add(cmd.Context(), args)
		if err != nil {
			return err
		}
		for _, id := range res.Skipped {
			tui.Infof(ios, "%s is already declared by the profile; skipped", id)
		}
		added := len(args) - len(res.Skipped)
		if added == 0 {
			tui.Successf(ios, "Profile already declares %s; installed the profile from %s",
				strings.Join(args, ", "), client.Dir)
		} else {
			tui.Successf(ios, "Added %d entr%s to %s", added, plural(added, "y", "ies"), mise.ConfigPath(client.Dir))
		}
		return nil
	},
}

func init() {
	packagesCmd.AddCommand(packagesAddCmd)
}
