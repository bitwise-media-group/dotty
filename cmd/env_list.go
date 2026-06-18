// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var envListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the credential names in a namespace.",
	Long: `Print the key names stored in the namespace, one per line, sorted. Values are
never printed — use get, use, or run to read them.`,
	Example: `  dotty env list --namespace=aws`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		keys, err := newEnvStore(ios).Keys(cmd.Context(), envFlags.Namespace)
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			tui.Infof(ios, "No credentials in namespace %q", envFlags.Namespace)
			return nil
		}
		for _, k := range keys {
			fmt.Fprintln(ios.Out, k)
		}
		return nil
	},
}

func init() {
	envCmd.AddCommand(envListCmd)
}
