// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var envAddCmd = &cobra.Command{
	Use:   "add <KEY>",
	Short: "Store a credential in a namespace.",
	Long: `Store a credential under KEY in the namespace. With a terminal attached the
value is read from a hidden prompt; when input is piped, the value is read from
stdin (a single trailing newline is stripped). The value is never taken from a
flag, so it stays out of shell history and the process list.`,
	Example: `  dotty env add --namespace=aws AWS_ACCESS_KEY_ID
  printf '%s' "$TOKEN" | dotty env add --namespace=ci GITHUB_TOKEN`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		key := args[0]
		if err := env.ValidateKey(key); err != nil {
			return err
		}

		value, err := readSecret(ios, key)
		if err != nil {
			return err
		}

		if err := newEnvStore(ios).Set(cmd.Context(), envFlags.Namespace, key, value); err != nil {
			return err
		}
		tui.Successf(ios, "Stored %s in namespace %q", key, envFlags.Namespace)
		return nil
	},
}

// readSecret reads a secret value without exposing it: a masked prompt when
// stdin is a terminal, otherwise stdin verbatim with one trailing newline
// stripped (so `printf '%s' "$x"` and `echo "$x"` both work).
func readSecret(ios cli.IOStreams, key string) (string, error) {
	if cli.IsTerminal(ios.In) {
		value, err := tui.Password(ios, fmt.Sprintf("Value for %s", key), nil)
		if errors.Is(err, tui.ErrNotInteractive) {
			return "", errors.New("no terminal for the prompt; pipe the value on stdin")
		}
		return value, err
	}
	data, err := io.ReadAll(ios.In)
	if err != nil {
		return "", fmt.Errorf("read value from stdin: %w", err)
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r"), nil
}

func init() {
	envCmd.AddCommand(envAddCmd)
}
