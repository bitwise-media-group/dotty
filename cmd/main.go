// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Command dotty provides common utilities for operating a terminal-driven
// workflow and dotfiles: machine profiles, Brewfile management, hardware
// security-key aliases, and SSH signing keys on YubiKeys.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/version"
)

// RootFlags holds the global flags shared by every dotty command.
type RootFlags struct {
	// Profile selects the profile a command operates on; empty means the
	// active profile (the active-profile symlink).
	Profile string
}

var (
	rootFlags = RootFlags{}

	// logger writes diagnostics to stderr so stdout stays reserved for
	// command output (key material, proxied tool output).
	logger = slog.New(slog.NewTextHandler(os.Stderr, nil))

	rootCmd = &cobra.Command{
		Use:   "dotty <noun> <verb>",
		Short: "Utilities for a terminal-driven workflow and dotfiles.",
		Long: `dotty manages the moving parts of a terminal-centric machine setup:
system profiles that travel across machines, the Homebrew Brewfile that keeps
installs reproducible, named aliases for hardware security keys, and SSH
signing keys that live on those keys (including git commit signing).`,
		Example: `  dotty profile new --name=work
  dotty brewfile add --cask ghostty
  dotty security-key add --name=primary
  dotty signing-key new`,
		Version:       version.Version,
		SilenceUsage:  true, // errors are failures, not usage mistakes
		SilenceErrors: true, // main prints the error once
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&rootFlags.Profile, "profile", "",
		"profile to operate on (defaults to the active profile)")
	rootCmd.SetVersionTemplate(fmt.Sprintf("dotty %s\n", version.String()))
}

// newRunner builds the exec runner commands use to drive external tools.
func newRunner(ios cli.IOStreams) *cli.ExecRunner {
	return cli.NewExecRunner(ios, logger)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rootCmd.SetArgs(dispatchArgs(os.Args))
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "dotty: %v\n", err)
		var exitErr *cli.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code) // proxied child's code; git inspects it
		}
		os.Exit(1)
	}
}

// dispatchArgs rewrites the argv for git's SSH-signing entry points, which
// exec a single program with no shell and so cannot name a subcommand:
// gpg.ssh.program may point at the dotty binary itself (git always passes
// -Y first) or at a dotty-ssh-sign symlink.
func dispatchArgs(argv []string) []string {
	rest := argv[1:]
	if filepath.Base(argv[0]) == "dotty-ssh-sign" || (len(rest) > 0 && rest[0] == "-Y") {
		return append([]string{"signing-key", "sign"}, rest...)
	}
	return rest
}
