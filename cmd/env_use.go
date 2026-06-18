// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// EnvUseFlags holds the flags for `dotty env use`.
type EnvUseFlags struct {
	InFile  string
	OutFile string
}

var envUseFlags = EnvUseFlags{}

var envUseCmd = &cobra.Command{
	Use:   "use [--in-file=<file>] [--out-file=<file>]",
	Short: "Fill a template with credential references.",
	Long: `Replace every {{ dotty://<namespace>/<key> }} reference (and bare {{ KEY }}
resolved against --namespace) in a template with its value, the way op inject
does. The template is read from --in-file or stdin and written to --out-file
(created with 0600) or stdout. An unknown or malformed reference is an error,
and an --out-file is written atomically so a failed run leaves no partial file.`,
	Example: `  dotty env use --in-file=.env.tmpl --out-file=.env
  echo 'token={{ dotty://ci/GITHUB_TOKEN }}' | dotty env use`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()

		var (
			src []byte
			err error
		)
		if envUseFlags.InFile != "" {
			if src, err = os.ReadFile(envUseFlags.InFile); err != nil {
				return fmt.Errorf("read template: %w", err)
			}
		} else if src, err = io.ReadAll(ios.In); err != nil {
			return fmt.Errorf("read template from stdin: %w", err)
		}

		resolve := newEnvStore(ios).Resolver(cmd.Context(), envFlags.Namespace)
		out, err := env.Inject(string(src), resolve)
		if err != nil {
			return err
		}

		if envUseFlags.OutFile != "" {
			if err := cli.AtomicWriteFile(envUseFlags.OutFile, []byte(out), 0o600); err != nil {
				return err
			}
			tui.Successf(ios, "Wrote %s", envUseFlags.OutFile)
			return nil
		}
		fmt.Fprint(ios.Out, out)
		return nil
	},
}

func init() {
	envUseCmd.Flags().StringVar(&envUseFlags.InFile, "in-file", "", "template file to read (default: stdin)")
	envUseCmd.Flags().StringVar(&envUseFlags.OutFile, "out-file", "", "file to write, created with 0600 (default: stdout)")
	envCmd.AddCommand(envUseCmd)
}
