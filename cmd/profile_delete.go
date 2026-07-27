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

// ProfileDeleteFlags holds the flags for `dotty profile delete`.
type ProfileDeleteFlags struct {
	Activate string
	Yes      bool
}

var profileDeleteFlags = ProfileDeleteFlags{}

var profileDeleteCmd = &cobra.Command{
	Use:     "delete [<name>]",
	Aliases: []string{"rm"},
	Short:   "Delete a profile and the directory behind it.",
	Long: `Delete the named profile, or the active one when no name is given. A
profile's content lives in the dotfiles repository and the config directory
only links to it, so deleting removes both the
$XDG_CONFIG_HOME/dotty/<name> entry and the profiles/<name> directory it
resolves to — unlinking alone would leave the next dotty dotfiles link to put
it straight back. Commit that removal to finish the job.

The last profile cannot be deleted, and deleting the active one means naming
its replacement first: --activate does that, and without it dotty asks. dotty
confirms before removing anything on a terminal; --yes skips the prompt, which
is also what a non-interactive run does.`,
	Example: `  dotty profile delete work
  dotty profile delete --activate=personal
  dotty profile rm work --yes`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		configDir, err := cli.ConfigDir()
		if err != nil {
			return err
		}
		name, err := profileName(configDir, args)
		if err != nil {
			return err
		}
		if !profile.Exists(configDir, name) {
			return fmt.Errorf("profile %q: %w", name, profile.ErrNotFound)
		}
		// profile.Delete re-checks both invariants; checking them up front
		// means a doomed delete never gets as far as the confirmation.
		profiles, err := profile.List(configDir)
		if err != nil {
			return err
		}
		if len(profiles) <= 1 {
			return fmt.Errorf("profile %q: %w", name, profile.ErrLastProfile)
		}
		site, backing, err := profile.Locate(configDir, name)
		if err != nil {
			return err
		}
		active, err := profile.ActiveName(configDir)
		if err != nil && !errors.Is(err, profile.ErrNoActiveProfile) {
			return err
		}

		replacement, err := deleteReplacement(ios, configDir, name, active)
		if err != nil {
			return err
		}

		if ios.IsInteractive() && !profileDeleteFlags.Yes {
			removes := site
			if backing != "" {
				removes = fmt.Sprintf("%s and %s", site, backing)
			}
			ok, err := tui.Confirm(ios, fmt.Sprintf("Delete profile %q?", name), "Removes "+removes)
			if err != nil && !errors.Is(err, tui.ErrAborted) {
				return err
			}
			if !ok {
				tui.Infof(ios, "Aborted; nothing deleted")
				return nil
			}
		}

		// Activating first keeps the active-profile symlink pointed at a
		// profile that exists for the whole operation.
		if replacement != "" {
			if err := activateProfile(cmd.Context(), ios, configDir, replacement); err != nil {
				return err
			}
		}
		if err := profile.Delete(configDir, name); err != nil {
			return err
		}
		tui.Successf(ios, "Deleted profile %s (%s)", name, site)
		if backing != "" {
			tui.Infof(ios, "Removed %s from the dotfiles repository — commit the deletion", backing)
		}
		return nil
	},
}

func init() {
	profileDeleteCmd.Flags().StringVar(&profileDeleteFlags.Activate, "activate", "",
		"profile to activate in place of the deleted one")
	profileDeleteCmd.Flags().BoolVar(&profileDeleteFlags.Yes, "yes", false,
		"skip the confirmation prompt")
	profileCmd.AddCommand(profileDeleteCmd)
}

// deleteReplacement settles which profile to activate in place of the one
// being deleted: --activate when given (honoured whether or not the deleted
// profile is active, since naming it is an explicit request), otherwise a
// picklist when the delete would orphan the active-profile symlink. It
// returns "" when the delete leaves the active profile alone.
func deleteReplacement(ios cli.IOStreams, configDir, name, active string) (string, error) {
	replacement := profileDeleteFlags.Activate
	if replacement == "" {
		if name != active {
			return "", nil
		}
		if !ios.IsInteractive() {
			return "", fmt.Errorf(
				"profile %q is active; pass --activate=<name> to choose its replacement", name)
		}
		picked, err := pickProfile(ios, configDir,
			fmt.Sprintf("Deleting %s — activate which profile instead?", name), name)
		if errors.Is(err, tui.ErrAborted) {
			return "", errors.New("no replacement chosen; nothing deleted")
		}
		if err != nil {
			return "", err
		}
		replacement = picked
	}
	if replacement == name {
		return "", fmt.Errorf("profile %q cannot replace itself", name)
	}
	if !profile.Exists(configDir, replacement) {
		return "", fmt.Errorf("profile %q: %w", replacement, profile.ErrNotFound)
	}
	return replacement, nil
}
