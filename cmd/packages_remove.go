// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/mise"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// PackagesRemoveFlags holds the flags for `dotty packages remove`.
type PackagesRemoveFlags struct {
	Sync bool
}

var packagesRemoveFlags = PackagesRemoveFlags{}

var packagesRemoveCmd = &cobra.Command{
	Use:     "remove [--sync] [<id> ...]",
	Aliases: []string{"rm"},
	Short:   "Remove packages from the profile.",
	Long: `Remove one or more packages from the profile's config.toml, or pick several
interactively (a filterable checklist) when no id is given. Only entries in
config.toml can be removed here: an entry a component fragment under conf.d/
declares is managed by dotty init — deselect the component instead. Nothing
is uninstalled: removed entries stay on the machine until
` + "`dotty packages sync`" + ` removes what the profile no longer declares —
pass --sync to run it immediately.`,
	Example: `  dotty packages remove aqua:sharkdp/bat brew-cask:raycast
  dotty packages rm
  dotty packages remove --sync brew:colima`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		dir, err := resolvePackagesDir()
		if err != nil {
			return err
		}
		ids := args
		if len(ids) == 0 {
			if ids, err = pickRemovals(ios, dir); err != nil || len(ids) == 0 {
				return err
			}
		}
		client, err := newMiseClient(cmd.Context(), ios)
		if err != nil {
			return err
		}
		res, err := client.Remove(cmd.Context(), ids)
		if err != nil {
			return err
		}
		for _, id := range res.NotFound {
			tui.Infof(ios, "%s is not declared by the profile; skipped", id)
		}
		for _, id := range res.Managed {
			tui.Warnf(ios, "%s is declared by a component fragment under %s; deselect the component with `dotty init`",
				id, mise.ConfDDir(dir))
		}
		removed := len(ids) - len(res.NotFound) - len(res.Managed)
		if removed > 0 {
			tui.Successf(ios, "Removed %d entr%s from %s", removed, plural(removed, "y", "ies"), mise.ConfigPath(dir))
		}
		if packagesRemoveFlags.Sync {
			return packagesSyncCmd.RunE(cmd, nil)
		}
		if removed > 0 {
			tui.Infof(ios, "Removed entries stay installed until `dotty packages sync`")
		}
		return nil
	},
}

// pickRemovals offers a filterable checklist of the config.toml entries —
// the ones a remove can act on. An empty return with a nil error means
// there is nothing to remove: no entries, an aborted picker, or an empty
// selection.
func pickRemovals(ios cli.IOStreams, dir string) ([]string, error) {
	declared, err := mise.Declared(dir)
	if err != nil {
		return nil, err
	}
	var own []string
	for id, e := range declared {
		if e.File == mise.ConfigPath(dir) {
			own = append(own, id)
		}
	}
	slices.Sort(own)
	if len(own) == 0 {
		tui.Infof(ios, "No entries in %s", mise.ConfigPath(dir))
		if len(declared) > 0 {
			fragments := make(map[string]bool)
			for _, e := range declared {
				fragments[filepath.Base(e.File)] = true
			}
			tui.Infof(ios, "The profile's packages all come from component fragments (%v); "+
				"deselect components with `dotty init`", slices.Sorted(maps.Keys(fragments)))
		}
		return nil, nil
	}
	if !ios.IsInteractive() {
		return nil, errors.New("no terminal for the picker; pass ids to remove")
	}
	options := make([]tui.Option, len(own))
	for i, id := range own {
		options[i] = tui.Option{Label: id, Value: id}
	}
	chosen, err := tui.MultiSelect(ios, fmt.Sprintf("Remove which entries from %s?", mise.ConfigPath(dir)), options)
	if errors.Is(err, tui.ErrAborted) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(chosen) == 0 {
		tui.Infof(ios, "Nothing selected; nothing removed")
		return nil, nil
	}
	return chosen, nil
}

func init() {
	packagesRemoveCmd.Flags().BoolVar(&packagesRemoveFlags.Sync, "sync", false,
		"run `dotty packages sync` after removing")
	packagesCmd.AddCommand(packagesRemoveCmd)
}
