// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/brewfile"
	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// BrewfileRemoveFlags holds the flags for `dotty brewfile remove`: the shared
// per-kind type flags plus --mas (dump writes mas entries that add cannot
// create) and --sync.
type BrewfileRemoveFlags struct {
	BrewfileKindFlags
	MAS  bool
	Sync bool
}

var brewfileRemoveFlags = BrewfileRemoveFlags{}

// kind maps the set flag to its brewfile kind; --mas is remove-only, the rest
// resolve through the shared set.
func (f BrewfileRemoveFlags) kind() brewfile.Kind {
	if f.MAS {
		return brewfile.KindMAS
	}
	return f.BrewfileKindFlags.kind()
}

var brewfileRemoveCmd = &cobra.Command{
	Use:     "remove [--tap | --cask | --formula | ...] [<name> ...]",
	Aliases: []string{"rm"},
	Short:   "Remove entries from the Brewfile.",
	Long: `Remove one or more entries from the Brewfile, or pick several interactively
(a filterable checklist) when no name is given. Entries default to formulae;
pass a type flag for anything else. Trust grants recorded for removed
tap-qualified names (and taps) are revoked best-effort. Nothing is
uninstalled: removed entries stay on the machine until
` + "`dotty brewfile sync`" + ` removes what the Brewfile no longer lists —
pass --sync to run it immediately.`,
	Example: `  dotty brewfile remove ripgrep jq
  dotty brewfile remove --cask ghostty
  dotty brewfile rm --tap
  dotty brewfile remove --sync acme/tap/widget`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		path, err := resolveBrewfilePath()
		if err != nil {
			return err
		}
		kind := brewfileRemoveFlags.kind()
		names := args
		if len(names) == 0 {
			if names, err = pickRemovals(cmd, ios, path, kind); err != nil || len(names) == 0 {
				return err
			}
		}
		res, err := brewfile.Remove(cmd.Context(), newRunner(ios), path, kind, names)
		if err != nil {
			return err
		}
		for _, name := range res.NotFound {
			tui.Infof(ios, "%s %q is not in the Brewfile; skipped", kind, name)
		}
		for _, name := range res.NotUntrusted {
			tui.Warnf(ios, "could not revoke trust for %s %q; run `brew untrust --%s %s` by hand",
				kind, name, kind, name)
		}
		removed := len(names) - len(res.NotFound)
		if removed > 0 {
			tui.Successf(ios, "Removed %d %s entr%s from %s", removed, kind, plural(removed, "y", "ies"), path)
		}
		if brewfileRemoveFlags.Sync {
			return brewfileSyncCmd.RunE(cmd, nil)
		}
		if removed > 0 {
			tui.Infof(ios, "Removed entries stay installed until `dotty brewfile sync`")
		}
		return nil
	},
}

// pickRemovals offers a filterable checklist of the Brewfile's entries of
// kind. An empty return with a nil error means there is nothing to remove —
// no entries, an aborted picker, or an empty selection.
func pickRemovals(cmd *cobra.Command, ios cli.IOStreams, path string, kind brewfile.Kind) ([]string, error) {
	entries, err := brewfile.List(cmd.Context(), newRunner(ios), path, kind)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		tui.Infof(ios, "No %s entries in %s", kind, path)
		return nil, nil
	}
	if !ios.IsInteractive() {
		return nil, errors.New("no terminal for the picker; pass names to remove")
	}
	options := make([]tui.Option, len(entries))
	for i, entry := range entries {
		options[i] = tui.Option{Label: entry, Value: entry}
	}
	chosen, err := tui.MultiSelect(ios, fmt.Sprintf("Remove which %s entries?", kind), options)
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
	brewfileRemoveCmd.Flags().BoolVar(&brewfileRemoveFlags.MAS, "mas", false, "remove Mac App Store entries")
	registerKindFlags(brewfileRemoveCmd, &brewfileRemoveFlags.BrewfileKindFlags, "remove", "mas")
	brewfileRemoveCmd.Flags().BoolVar(&brewfileRemoveFlags.Sync, "sync", false,
		"run `dotty brewfile sync` after removing")
	brewfileCmd.AddCommand(brewfileRemoveCmd)
}
