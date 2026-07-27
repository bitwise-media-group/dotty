// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var profileListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the profiles available on this machine.",
	Long: `Print every profile under $XDG_CONFIG_HOME/dotty as a table — name,
description, and creation date — with * marking the active one. Unlike the
other profile verbs this one never prompts, so the output is the same piped as
it is on a terminal; ask for one profile's detail with get.`,
	Example: `  dotty profile list
  dotty profile ls`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		configDir, err := cli.ConfigDir()
		if err != nil {
			return err
		}
		profiles, err := profile.List(configDir)
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			tui.Infof(ios, "No profiles yet; run `dotty profile new`")
			return nil
		}
		// A machine mid-setup has profiles but no active-profile symlink yet;
		// that marks no row rather than failing the listing.
		active, err := profile.ActiveName(configDir)
		if err != nil && !errors.Is(err, profile.ErrNoActiveProfile) {
			return err
		}

		rows := make([]tui.TableRow, len(profiles))
		for i, p := range profiles {
			marker := " "
			if p.Name == active {
				marker = "*"
			}
			created := ""
			if !p.CreatedAt.IsZero() {
				created = p.CreatedAt.Format(time.DateOnly)
			}
			rows[i] = tui.TableRow{
				Cells: []string{marker, p.Name, p.Description, created},
				Value: p.Name,
			}
		}
		_, _ = fmt.Fprint(ios.Out, tui.RenderTable([]string{"", "NAME", "DESCRIPTION", "CREATED"}, rows))
		return nil
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd)
}
