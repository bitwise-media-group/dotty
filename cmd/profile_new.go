// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/tui"
	"github.com/bitwise-media-group/dotty/internal/wizard"
)

var profileNewFlags = wizard.Flags{}

var profileNewCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a profile and set this machine up as one of its class.",
	Long: `Create a profile by running the init interview for it: a profile directory
with nothing in it is a machine class that configures nothing, so new is
dotty init with the profile name settled in advance. Without --name dotty
prompts for one, then asks init's questions — where the dotfiles repository
lives, which add-ons and coding agents to include, security keys, macOS
defaults — renders profiles/<name> into the repository, links the home/ tree,
and activates the profile. Nothing is written until the summary is confirmed.

Every init flag works here too, so a profile can be created unattended. The
name has to be free: an existing profile is re-rendered with dotty init and
switched to with dotty profile activate.`,
	Example: `  dotty profile new
  dotty profile new --name=work --description="work laptop"
  dotty profile new --name=work --addons=tmux --agents=claude-code --yes`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return createProfile(cmd.Context(), cli.System(), interviewFlags(cmd, profileNewFlags))
	},
}

func init() {
	profileNewCmd.Flags().StringVar(&profileNewFlags.ProfileName, "name", "", "name for the new profile")
	registerInterviewFlags(profileNewCmd, &profileNewFlags)
	// init activates what it renders, so the flag has nothing left to decide.
	_ = profileNewCmd.Flags().Bool("activate", false, "activate the profile after creating it")
	_ = profileNewCmd.Flags().MarkDeprecated("activate", "profile new always activates the new profile")
	profileCmd.AddCommand(profileNewCmd)
}

// createProfile creates a profile the only way a useful one comes about: the
// init flow with the profile named up front. The activate command's
// create-when-missing path calls it too, for the name it was asked to
// activate but could not find.
func createProfile(ctx context.Context, ios cli.IOStreams, flags wizard.Flags) error {
	// The names to refuse: this machine's profiles and the selected
	// repository's, since either one is an existing machine class.
	taken := wizard.ExistingProfiles(flags)
	if flags.ProfileName == "" {
		name, err := tui.Input(ios, "Profile name", "work", func(s string) error {
			if err := profile.ValidateName(s); err != nil {
				return err
			}
			if slices.Contains(taken, s) {
				return fmt.Errorf("profile %q already exists", s)
			}
			return nil
		})
		if errors.Is(err, tui.ErrNotInteractive) {
			return errors.New("no profile name given; pass --name or run interactively")
		}
		if errors.Is(err, tui.ErrAborted) {
			return nil // esc backs out before anything is written
		}
		if err != nil {
			return err
		}
		flags.ProfileName = name
	}
	if err := profile.ValidateName(flags.ProfileName); err != nil {
		return err
	}
	if slices.Contains(taken, flags.ProfileName) {
		return fmt.Errorf("profile %q: %w (re-render it with `dotty init`, or switch to it with"+
			" `dotty profile activate`)", flags.ProfileName, profile.ErrExists)
	}
	return runInit(ctx, ios, flags)
}
