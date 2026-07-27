// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// profileCmd groups the profile verbs.
var profileCmd = &cobra.Command{
	Use:   "profile <verb>",
	Short: "Manage system profiles that travel across machines.",
	Long: `Profiles are per-machine configuration sets — a Brewfile today; prompt and
terminal themes later — stored under $XDG_CONFIG_HOME/dotty/<name> so a public
dotfiles repository can carry them. One profile is active at a time, named by
the active-profile symlink.`,
	Example: `  dotty profile new --name=work --description="work laptop"
  dotty profile list
  dotty profile activate
  dotty profile activate --name=personal`,
}

func init() {
	rootCmd.AddCommand(profileCmd)
}

// profileName resolves which profile a verb operates on: the positional
// argument, else the global --profile flag, else the active profile.
func profileName(configDir string, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if rootFlags.Profile != "" {
		return rootFlags.Profile, nil
	}
	name, err := profile.ActiveName(configDir)
	if errors.Is(err, profile.ErrNoActiveProfile) {
		return "", fmt.Errorf("%w, or name the profile", err)
	}
	return name, err
}

// pickProfile fuzzy-picks a profile, labelling each with its description.
// skip drops one name from the list — the profile being replaced cannot also
// be its own replacement; pass "" to offer all of them.
func pickProfile(ios cli.IOStreams, configDir, title, skip string) (string, error) {
	profiles, err := profile.List(configDir)
	if err != nil {
		return "", err
	}
	options := make([]tui.Option, 0, len(profiles))
	for _, p := range profiles {
		if p.Name == skip {
			continue
		}
		label := p.Name
		if p.Description != "" {
			label = fmt.Sprintf("%s — %s", p.Name, p.Description)
		}
		options = append(options, tui.Option{Label: label, Value: p.Name})
	}
	if len(options) == 0 {
		return "", errors.New("no profiles exist yet; run `dotty profile new`")
	}
	return tui.FuzzySelect(ios, title, options)
}
