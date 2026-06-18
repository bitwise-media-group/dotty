// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
)

// EnvGetFlags holds the flags for `dotty env get`.
type EnvGetFlags struct {
	NoNewline bool
}

var envGetFlags = EnvGetFlags{}

var envGetCmd = &cobra.Command{
	Use:   "get <KEY | dotty://namespace/key>",
	Short: "Print a credential value.",
	Long: `Print the value of a single credential to stdout. The argument is either a
KEY in the --namespace, or a full dotty://<namespace>/<key> reference (which
names its own namespace). A trailing newline is printed unless --no-newline.`,
	Example: `  dotty env get --namespace=aws AWS_ACCESS_KEY_ID
  dotty env get dotty://aws/AWS_ACCESS_KEY_ID | pbcopy`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		ref, err := env.ParseRef(args[0])
		if err != nil {
			return err
		}
		value, err := newEnvStore(ios).Resolver(cmd.Context(), envFlags.Namespace)(ref.Namespace, ref.Key)
		if err != nil {
			return err
		}
		if envGetFlags.NoNewline {
			fmt.Fprint(ios.Out, value)
		} else {
			fmt.Fprintln(ios.Out, value)
		}
		return nil
	},
}

func init() {
	envGetCmd.Flags().BoolVar(&envGetFlags.NoNewline, "no-newline", false, "do not print a trailing newline")
	envCmd.AddCommand(envGetCmd)
}
