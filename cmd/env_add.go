// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// EnvAddFlags holds the flags for `dotty env add`.
type EnvAddFlags struct {
	InFile  string
	OutFile string
}

var envAddFlags = EnvAddFlags{}

var envAddCmd = &cobra.Command{
	Use:   "add [<KEY>]",
	Short: "Store a credential, or capture a whole .env file.",
	Long: `Store a credential under KEY in the namespace. With a terminal attached the
value is read from a hidden prompt; when input is piped, the value is read from
stdin (a single trailing newline is stripped). The value is never taken from a
flag, so it stays out of shell history and the process list.

With --in-file, KEY is omitted and a .env file is captured instead: every
KEY=value assignment is stored in the namespace and its value is replaced with a
{{ dotty://<namespace>/KEY }} reference (the inverse of env use). The result is
written to --out-file, which defaults to --in-file; replacing an existing file
is confirmed first. Blank lines, comments, empty values, and values that are
already references are left untouched.`,
	Example: `  dotty env add --namespace=aws AWS_ACCESS_KEY_ID
  printf '%s' "$TOKEN" | dotty env add --namespace=ci GITHUB_TOKEN
  dotty env add --namespace=aws --in-file=.env`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()

		if envAddFlags.InFile != "" {
			if len(args) > 0 {
				return errors.New("KEY cannot be combined with --in-file")
			}
			return addFromFile(cmd.Context(), ios, envFlags.Namespace, envAddFlags.InFile, envAddFlags.OutFile)
		}
		if len(args) != 1 {
			return errors.New("requires a KEY argument, or --in-file to capture a .env file")
		}

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

// addFromFile captures every secret in the .env file at inFile into namespace
// and rewrites the file with references in their place. outFile defaults to
// inFile; an existing destination is overwritten only after confirmation.
// Secrets are stored before the file is written, so a write failure still
// leaves the values safely in the keychain.
func addFromFile(ctx context.Context, ios cli.IOStreams, namespace, inFile, outFile string) error {
	src, err := os.ReadFile(inFile)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}
	out, secrets, err := env.Capture(string(src), namespace)
	if err != nil {
		return err
	}
	if len(secrets) == 0 {
		return fmt.Errorf("no secret values found in %s", inFile)
	}

	if outFile == "" {
		outFile = inFile
	}
	if _, err := os.Stat(outFile); err == nil {
		proceed, err := confirmOverwrite(ios, outFile)
		if err != nil {
			return err
		}
		if !proceed {
			tui.Infof(ios, "Aborted; no changes made")
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", outFile, err)
	}

	if err := newEnvStore(ios).SetAll(ctx, namespace, secrets); err != nil {
		return err
	}
	if err := cli.AtomicWriteFile(outFile, []byte(out), 0o600); err != nil {
		return err
	}

	noun := "secret"
	if len(secrets) != 1 {
		noun += "s"
	}
	tui.Successf(ios, "Captured %d %s into namespace %q and wrote %s", len(secrets), noun, namespace, outFile)
	return nil
}

// confirmOverwrite asks before clobbering an existing destination. Outside a
// terminal there is no way to ask, so it refuses rather than overwrite
// silently and points at the safe alternative.
func confirmOverwrite(ios cli.IOStreams, path string) (bool, error) {
	ok, err := tui.Confirm(ios,
		fmt.Sprintf("Overwrite %s?", path),
		"Its values will be stored in the keychain and replaced with references.")
	if errors.Is(err, tui.ErrNotInteractive) {
		return false, fmt.Errorf(
			"refusing to overwrite %s without confirmation; re-run in a terminal or set --out-file to a new path", path)
	}
	return ok, err
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
	envAddCmd.Flags().StringVar(&envAddFlags.InFile, "in-file", "",
		"capture secrets from this .env file instead of a single KEY")
	envAddCmd.Flags().StringVar(&envAddFlags.OutFile, "out-file", "",
		"file to write captured references to (default: --in-file)")
	envCmd.AddCommand(envAddCmd)
}
