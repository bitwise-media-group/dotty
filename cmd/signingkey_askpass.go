// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/signingkey"
)

// signingKeyAskPassCmd is the $SSH_ASKPASS bridge that turns an OpenSSH FIDO PIN
// prompt into a pinentry-mac dialog. dotty's own sign path points ssh-keygen at
// it with the DOTTY_ASKPASS=1 sentinel; a globally-exported
// SSH_ASKPASS=<dir>/dotty-ssh-askpass routes every other PIN prompt here by
// argv[0] basename. Either way the argv dispatcher rewrites the call, so it is
// never run by hand, hence hidden.
var signingKeyAskPassCmd = &cobra.Command{
	Use:    "ask-pass [prompt]",
	Short:  "Bridge OpenSSH PIN prompts to pinentry-mac (internal).",
	Hidden: true,
	// ssh passes a single prompt that may begin with '-'; take it verbatim
	// rather than parsing it as flags.
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		var prompt string
		if len(args) > 0 {
			prompt = args[0]
		}
		reply := signingkey.AskPassReply(cmd.Context(), newRunner(ios), prompt, os.Getenv(signingkey.KeyInfoEnv), os.ReadFile)
		_, _ = fmt.Fprintln(ios.Out, reply)
		return nil
	},
}

func init() {
	signingKeyCmd.AddCommand(signingKeyAskPassCmd)
}
