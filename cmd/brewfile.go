// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/profile"
)

// brewfileCmd groups the brewfile verbs.
var brewfileCmd = &cobra.Command{
	Use:     "brewfile <verb>",
	Aliases: []string{"brew"},
	Short:   "Manage the profile's Brewfile for reproducible brews.",
	Long: `Maintain a homebrew bundle Brewfile so a machine's brews stay reproducible
on and across systems. Commands operate on the active profile's Brewfile, or
on a specific profile's via the global --profile flag.`,
	Example: `  dotty brewfile add ripgrep
  dotty brewfile add --cask ghostty
  dotty --profile=work brewfile sync
  dotty brew upgrade`,
}

func init() {
	rootCmd.AddCommand(brewfileCmd)
}

// resolveBrewfilePath finds the Brewfile the brewfile verbs operate on: the
// --profile flag's profile when given (it must exist), otherwise the active
// profile.
func resolveBrewfilePath() (string, error) {
	configDir, err := cli.ConfigDir()
	if err != nil {
		return "", err
	}
	if rootFlags.Profile != "" {
		if !profile.Exists(configDir, rootFlags.Profile) {
			return "", fmt.Errorf("profile %q: %w", rootFlags.Profile, profile.ErrNotFound)
		}
		return profile.BrewfilePath(profile.Dir(configDir, rootFlags.Profile)), nil
	}
	dir, err := profile.ActiveDir(configDir)
	if errors.Is(err, profile.ErrNoActiveProfile) {
		return "", fmt.Errorf("%w, or pass --profile", err)
	}
	if err != nil {
		return "", err
	}
	return profile.BrewfilePath(dir), nil
}
