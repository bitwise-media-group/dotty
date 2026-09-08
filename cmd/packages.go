// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/mise"
	"github.com/bitwise-media-group/dotty/internal/profile"
)

// packagesCmd groups the packages verbs.
var packagesCmd = &cobra.Command{
	Use:     "packages <verb>",
	Aliases: []string{"pkg"},
	Short:   "Manage the profile's packages (mise tools and bootstrap packages).",
	Long: `Maintain the profile's mise directory — config.toml, the component
fragments under conf.d/, and mise.lock — so a machine's packages stay
reproducible on and across systems. Two kinds of entry live there: tools
([tools], locked per platform in mise.lock, installed by mise itself) and
bootstrap packages ([bootstrap.packages], poured into the Homebrew prefix or
the OS package manager at their latest version). Commands operate on the
active profile, or on a specific profile's via the global --profile flag;
mise is installed into ~/.local/bin when the machine has none.

Package ids are full backend ids — aqua:sharkdp/bat, github:owner/repo,
npm:prettier, brew:git, brew-cask:ghostty — never bare registry names, which
resolve to whichever registry entry claims them.`,
	Example: `  dotty packages add aqua:sharkdp/bat brew-cask:raycast
  dotty packages sync
  dotty --profile=work packages status
  dotty pkg upgrade`,
}

func init() {
	rootCmd.AddCommand(packagesCmd)
}

// resolvePackagesDir finds the mise directory the packages verbs operate
// on: the --profile flag's profile when given (it must exist), otherwise the
// active profile. A profile with no config.toml yet — hand-made, or from
// before packages moved to mise — gets the default one.
func resolvePackagesDir() (string, error) {
	configDir, err := cli.ConfigDir()
	if err != nil {
		return "", err
	}
	var profileDir string
	if rootFlags.Profile != "" {
		if !profile.Exists(configDir, rootFlags.Profile) {
			return "", fmt.Errorf("profile %q: %w", rootFlags.Profile, profile.ErrNotFound)
		}
		profileDir = profile.Dir(configDir, rootFlags.Profile)
	} else {
		profileDir, err = profile.ActiveDir(configDir)
		if errors.Is(err, profile.ErrNoActiveProfile) {
			return "", fmt.Errorf("%w, or pass --profile", err)
		}
		if err != nil {
			return "", err
		}
	}
	dir := mise.Dir(profileDir)
	if err := mise.EnsureConfig(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// newMiseClient resolves the profile's mise directory and the mise binary
// (installing it when missing) and returns a client bound to both.
func newMiseClient(ctx context.Context, ios cli.IOStreams) (*mise.Client, error) {
	dir, err := resolvePackagesDir()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	runner := newRunner(ios)
	bin, err := mise.EnsureInstalled(ctx, runner, exec.LookPath, mise.Fetch, home)
	if err != nil {
		return nil, err
	}
	return mise.New(runner, bin, dir), nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
