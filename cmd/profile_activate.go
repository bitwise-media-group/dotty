// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// ProfileActivateFlags holds the flags for `dotty profile activate`.
type ProfileActivateFlags struct {
	Name string
}

var profileActivateFlags = ProfileActivateFlags{}

var profileActivateCmd = &cobra.Command{
	Use:   "activate",
	Short: "Activate an existing profile.",
	Long: `Point the active-profile symlink at a profile. Without --name dotty
presents a fuzzy-finding picklist of existing profiles. If the named profile
does not exist, dotty offers to create it first. A freshly activated profile
with no Brewfile gets one dumped from the currently installed brews.`,
	Example: `  dotty profile activate
  dotty profile activate --name=work`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		configDir, err := cli.ConfigDir()
		if err != nil {
			return err
		}

		name := profileActivateFlags.Name
		if name == "" {
			name, err = pickProfile(ios, configDir, "Activate which profile?", "")
			if errors.Is(err, tui.ErrAborted) {
				return nil // esc backs out without changing anything
			}
			if errors.Is(err, tui.ErrNotInteractive) {
				return errors.New("no profile name given; pass --name or run interactively")
			}
			if err != nil {
				return err
			}
		}

		if !profile.Exists(configDir, name) {
			if !ios.IsInteractive() {
				return fmt.Errorf("profile %q: %w", name, profile.ErrNotFound)
			}
			ok, err := tui.Confirm(ios, fmt.Sprintf("Profile %q does not exist. Create it?", name), "")
			if err != nil && !errors.Is(err, tui.ErrAborted) {
				return err
			}
			if !ok {
				return fmt.Errorf("profile %q: %w", name, profile.ErrNotFound)
			}
			return createProfile(cmd.Context(), ios, name, "", true)
		}

		return activateProfile(cmd.Context(), ios, configDir, name)
	},
}

func init() {
	profileActivateCmd.Flags().StringVar(&profileActivateFlags.Name, "name", "", "profile to activate")
	profileCmd.AddCommand(profileActivateCmd)
}

// activateProfile swaps the active-profile symlink; profile.Activate dumps a
// Brewfile when the profile has none yet, so the notice is printed up front.
func activateProfile(ctx context.Context, ios cli.IOStreams, configDir, name string) error {
	fresh := false
	if _, err := os.Stat(profile.BrewfilePath(profile.Dir(configDir, name))); errors.Is(err, fs.ErrNotExist) {
		fresh = true
		tui.Infof(ios, "Profile %s has no Brewfile yet — dumping the installed brews", name)
	}
	if _, err := profile.Activate(ctx, newRunner(ios), configDir, name); err != nil {
		return err
	}
	tui.Successf(ios, "Activated profile %s", name)
	if fresh {
		tui.Successf(ios, "Wrote %s", profile.BrewfilePath(profile.Dir(configDir, name)))
	}
	return nil
}
