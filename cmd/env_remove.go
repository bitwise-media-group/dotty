// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// EnvRemoveFlags holds the flags for `dotty env remove`.
type EnvRemoveFlags struct {
	All bool
}

var envRemoveFlags = EnvRemoveFlags{}

var envRemoveCmd = &cobra.Command{
	Use:     "remove [<KEY>]",
	Aliases: []string{"rm"},
	Short:   "Remove credentials from a namespace.",
	Long: `Remove one credential by KEY, the whole namespace with --all, or pick several
interactively (a filterable checklist) when no KEY is given. Removing the last
credential also removes the namespace's keychain item.`,
	Example: `  dotty env remove --namespace=aws AWS_ACCESS_KEY_ID
  dotty env remove --namespace=aws --all
  dotty env rm --namespace=aws`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		store := newEnvStore(ios)
		ns := envFlags.Namespace

		if envRemoveFlags.All {
			if len(args) > 0 {
				return errors.New("pass either a KEY or --all, not both")
			}
			return removeNamespace(cmd, ios, store, ns)
		}

		if len(args) == 1 {
			found, err := store.Delete(cmd.Context(), ns, args[0])
			if err != nil {
				return err
			}
			if !found {
				tui.Infof(ios, "No credential %q in namespace %q", args[0], ns)
				return nil
			}
			tui.Successf(ios, "Removed %s from namespace %q", args[0], ns)
			return nil
		}

		return removeInteractive(cmd, ios, store, ns)
	},
}

// removeInteractive offers a filterable checklist of a namespace's keys.
func removeInteractive(cmd *cobra.Command, ios cli.IOStreams, store *env.Store, ns string) error {
	keys, err := store.Keys(cmd.Context(), ns)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		tui.Infof(ios, "No credentials in namespace %q", ns)
		return nil
	}
	if !ios.IsInteractive() {
		return errors.New("no terminal for the picker; pass a KEY or --all")
	}

	options := make([]tui.Option, len(keys))
	for i, k := range keys {
		options[i] = tui.Option{Label: k, Value: k}
	}
	chosen, err := tui.MultiSelect(ios, fmt.Sprintf("Remove which credentials from %q?", ns), options)
	if errors.Is(err, tui.ErrAborted) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		tui.Infof(ios, "Nothing selected; nothing removed")
		return nil
	}
	for _, k := range chosen {
		if _, err := store.Delete(cmd.Context(), ns, k); err != nil {
			return err
		}
	}
	tui.Successf(ios, "Removed %d credential%s from namespace %q", len(chosen), plural(len(chosen), "", "s"), ns)
	return nil
}

// removeNamespace drops a whole namespace, confirming first when interactive.
func removeNamespace(cmd *cobra.Command, ios cli.IOStreams, store *env.Store, ns string) error {
	keys, err := store.Keys(cmd.Context(), ns)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		tui.Infof(ios, "No credentials in namespace %q", ns)
		return nil
	}
	if ios.IsInteractive() {
		title := fmt.Sprintf("Remove all %d credential%s in namespace %q?", len(keys), plural(len(keys), "", "s"), ns)
		ok, err := tui.Confirm(ios, title, "")
		if err != nil && !errors.Is(err, tui.ErrAborted) {
			return err
		}
		if !ok {
			tui.Infof(ios, "Aborted; nothing removed")
			return nil
		}
	}
	if err := store.DeleteNamespace(cmd.Context(), ns); err != nil {
		return err
	}
	tui.Successf(ios, "Removed namespace %q (%d credential%s)", ns, len(keys), plural(len(keys), "", "s"))
	return nil
}

func init() {
	envRemoveCmd.Flags().BoolVar(&envRemoveFlags.All, "all", false, "remove the entire namespace")
	envCmd.AddCommand(envRemoveCmd)
}
