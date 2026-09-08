// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/scaffold"
	"github.com/bitwise-media-group/dotty/internal/tui"
	"github.com/bitwise-media-group/dotty/internal/wizard"
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
does not exist, dotty offers to create it first, which runs the init
interview for it the way dotty profile new does — and ends with the new
profile active. Everything reached through the active-profile link swaps
with it, the profile's packages included: ~/.config/mise now names the new
profile's mise directory, and dotty packages sync converges the machine on
it.`,
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
			return createProfile(cmd.Context(), ios, wizard.Flags{ProfileName: name})
		}

		return activateProfile(cmd.Context(), ios, configDir, name)
	},
}

func init() {
	profileActivateCmd.Flags().StringVar(&profileActivateFlags.Name, "name", "", "profile to activate")
	profileCmd.AddCommand(profileActivateCmd)
}

// activateProfile swaps the active-profile symlink and reminds that the
// machine's packages follow the profile only once synced.
func activateProfile(ctx context.Context, ios cli.IOStreams, configDir, name string) error {
	if _, err := profile.Activate(configDir, name); err != nil {
		return err
	}
	tui.Successf(ios, "Activated profile %s", name)
	tui.Infof(ios, "Converge the machine's packages with: dotty packages sync")
	activatePrivate(ctx, ios, configDir, name)
	return nil
}

// activatePrivate swaps the private identity along with the profile: when
// the new profile keeps a private repository, its tree is materialized and
// relinked best-effort — activation must not fail because a security key is
// unplugged, and `dotty private link` finishes the job later.
func activatePrivate(ctx context.Context, ios cli.IOStreams, configDir, name string) {
	answers, err := scaffold.LoadAnswers(profile.Dir(configDir, name))
	if err != nil {
		return // a profile without answers keeps no private repository
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	repo := privateRepoFromAnswers(answers, home)
	if repo == "" {
		return
	}
	if !privdot.IsRepo(repo) {
		tui.Warnf(ios, "Private repository %s is not scaffolded; run dotty private init", repo)
		return
	}
	if err := runPrivateLink(ctx, ios, repo, name, "", false); err != nil {
		tui.Warnf(ios, "Private dotfiles not relinked: %v (retry with dotty private link)", err)
	}
}
