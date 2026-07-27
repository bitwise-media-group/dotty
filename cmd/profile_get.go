// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/profile"
)

// ProfileGetFlags holds the flags for `dotty profile get`.
type ProfileGetFlags struct {
	Format string
}

var profileGetFlags = ProfileGetFlags{}

var profileGetCmd = &cobra.Command{
	Use:   "get [<name>]",
	Short: "Show a profile's metadata and where it lives.",
	Long: `Print a profile's metadata — name, description, creation date — alongside the
machine state around it: the profile directory, the dotfiles repository
directory it links to, whether it is the active profile, and how many entries
its Brewfile carries. Without a name dotty describes the active profile, or
the one the global --profile names.

--format=json prints profile.json verbatim instead, which for a profile dotty
init built also carries the wizard answers. That file knows nothing about this
machine, so the link target and active flag are text-mode only.`,
	Example: `  dotty profile get
  dotty profile get work
  dotty profile get work --format=json`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		if profileGetFlags.Format != "text" && profileGetFlags.Format != "json" {
			return fmt.Errorf("unknown format %q (expected text or json)", profileGetFlags.Format)
		}
		configDir, err := cli.ConfigDir()
		if err != nil {
			return err
		}
		name, err := profileName(configDir, args)
		if err != nil {
			return err
		}
		p, err := profile.Load(configDir, name)
		if err != nil {
			return err
		}
		site, backing, err := profile.Locate(configDir, name)
		if err != nil {
			return err
		}
		if profileGetFlags.Format == "json" {
			return printProfileJSON(ios, site, p)
		}

		active, err := profile.ActiveName(configDir)
		if err != nil && !errors.Is(err, profile.ErrNoActiveProfile) {
			return err
		}
		entries, err := brewfileEntries(profile.BrewfilePath(site))
		if err != nil {
			return err
		}

		fields := [][2]string{{"NAME", p.Name}}
		if p.Description != "" {
			fields = append(fields, [2]string{"DESCRIPTION", p.Description})
		}
		if !p.CreatedAt.IsZero() {
			fields = append(fields, [2]string{"CREATED", p.CreatedAt.Format(time.DateOnly)})
		}
		fields = append(fields, [2]string{"PATH", site})
		if backing != "" {
			fields = append(fields, [2]string{"LINKS TO", backing})
		}
		fields = append(fields,
			[2]string{"ACTIVE", yesNo(name == active)},
			[2]string{"BREWFILE", brewfileSummary(entries)},
		)
		for _, f := range fields {
			_, _ = fmt.Fprintf(ios.Out, "%-11s  %s\n", f[0], f[1])
		}
		return nil
	},
}

func init() {
	profileGetCmd.Flags().StringVar(&profileGetFlags.Format, "format", "text",
		"output format: text (metadata plus machine state) or json (profile.json verbatim)")
	profileCmd.AddCommand(profileGetCmd)
}

// printProfileJSON writes the profile's profile.json as it sits on disk. A
// hand-copied profile directory has no such file, so the metadata Load
// synthesised from the directory name stands in — scripts get an object
// either way.
func printProfileJSON(ios cli.IOStreams, site string, p profile.Profile) error {
	data, err := os.ReadFile(profile.MetadataPath(site))
	if errors.Is(err, fs.ErrNotExist) {
		if data, err = json.MarshalIndent(p, "", "  "); err != nil {
			return fmt.Errorf("encode profile metadata: %w", err)
		}
		data = append(data, '\n')
	} else if err != nil {
		return fmt.Errorf("read profile metadata: %w", err)
	}
	_, _ = fmt.Fprint(ios.Out, string(data))
	return nil
}

// brewfileEntries counts a Brewfile's declarations, -1 when the profile has no
// Brewfile yet. brew bundle writes one entry per line, so the non-blank,
// non-comment lines are the entries.
func brewfileEntries(path string) (int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return -1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read Brewfile %s: %w", path, err)
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			count++
		}
	}
	return count, nil
}

// brewfileSummary renders the count brewfileEntries returned.
func brewfileSummary(entries int) string {
	if entries < 0 {
		return "none"
	}
	return fmt.Sprintf("%d entr%s", entries, plural(entries, "y", "ies"))
}

// yesNo renders a boolean the way the get tables read it.
func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
