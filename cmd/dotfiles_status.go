// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import "github.com/spf13/cobra"

var dotfilesStatusCmd = &cobra.Command{
	Use:                "status [mise arguments]",
	Aliases:            []string{"ls"},
	Short:              "Show mise's dotfile deployment and tracking status.",
	Long:               "Report managed symlinks, copies, tracked sources, and history using mise dot status.",
	Example:            "  dotty dotfiles status\n  dotty dotfiles status --json",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return proxyDotfiles(cmd, append([]string{"status"}, args...))
	},
}

func init() { dotfilesCmd.AddCommand(dotfilesStatusCmd) }
