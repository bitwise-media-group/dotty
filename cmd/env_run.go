// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"sort"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// envRunOwnFlags is the ExtractFlags spec for the run proxy: the flags dotty
// owns. Everything else — the command and its own flags — forwards verbatim.
var envRunOwnFlags = map[string]bool{"namespace": true}

var envRunCmd = &cobra.Command{
	Use:   "run -- <command> [args...]",
	Short: "Run a command with a namespace's credentials in its environment.",
	Long: `Launch a command with every credential in the namespace exported as an
environment variable, the way op run does. dotty parses its own --namespace
(and --help); everything after -- is the command and its arguments, passed
through untouched. Put dotty's flags before -- (use -- when the command takes a
--namespace of its own). The command inherits the terminal, and dotty exits
with its exit code.`,
	Example: `  dotty env run --namespace=aws -- aws s3 ls
  dotty env run --namespace=ci -- ./deploy.sh`,
	// Verbatim passthrough: the child owns its flags, so parsing is off and
	// --help is handled manually via ExtractFlags.
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		own, rest, help := cli.ExtractFlags(args, envRunOwnFlags)
		if help {
			return cmd.Help()
		}
		ios := cli.System()

		namespace := envFlags.Namespace
		if ns, ok := own["namespace"]; ok {
			namespace = ns
		}

		if len(rest) > 0 && rest[0] == "--" {
			rest = rest[1:]
		}
		if len(rest) == 0 {
			return errors.New("no command given; usage: dotty env run --namespace=<ns> -- <command> [args...]")
		}

		secrets, err := newEnvStore(ios).All(cmd.Context(), namespace)
		if err != nil {
			return err
		}
		extraEnv := make([]string, 0, len(secrets))
		for k, v := range secrets {
			extraEnv = append(extraEnv, k+"="+v)
		}
		sort.Strings(extraEnv) // deterministic order; later duplicates would win regardless

		return newRunner(ios).RunInteractiveEnv(cmd.Context(), extraEnv, rest[0], rest[1:]...)
	},
}

func init() {
	envCmd.AddCommand(envRunCmd)
}
