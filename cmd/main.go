// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

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
	"github.com/bitwise-media-group/dotty/internal/signingkey"
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

	rootCmd.SetArgs(dispatchArgs(os.Args, os.Getenv))
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "dotty: %v\n", err)
		if exitErr, ok := errors.AsType[*cli.ExitError](err); ok {
			os.Exit(exitErr.Code) // proxied child's code; git inspects it
		}
		os.Exit(1)
	}
}

// dispatchArgs rewrites the argv for the SSH entry points that exec a single
// program with no shell and so cannot name a subcommand. A PIN prompt argument
// can't be told apart from a mistyped command, so an $SSH_ASKPASS invocation is
// recognized two ways, either routing to `signing-key ask-pass`: the
// DOTTY_ASKPASS=1 sentinel dotty's own sign path sets on its ssh-keygen child,
// or a dotty-ssh-askpass argv[0] — the basename a globally-exported SSH_ASKPASS
// points at, so every OpenSSH PIN prompt routes here, including the ssh-keygen
// that `signing-key new` and import spawn (which inherits no sentinel).
// Otherwise gpg.ssh.program is either the dotty binary (git always passes -Y
// first) or a dotty-ssh-sign symlink, routing to `signing-key sign`.
func dispatchArgs(argv []string, getenv func(string) string) []string {
	rest := argv[1:]
	base := filepath.Base(argv[0])
	switch {
	case getenv(signingkey.AskPassEnv) == "1" || base == "dotty-ssh-askpass":
		return append([]string{"signing-key", "ask-pass"}, rest...)
	case base == "dotty-ssh-sign" || (len(rest) > 0 && rest[0] == "-Y"):
		return append([]string{"signing-key", "sign"}, rest...)
	}
	return rest
}
