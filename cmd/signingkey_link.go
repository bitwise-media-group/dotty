// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/securitykey"
	"github.com/bitwise-media-group/dotty/internal/signingkey"
)

var signingKeyLinkCmd = &cobra.Command{
	Use:   "link [path]",
	Short: "Symlink a stable path at the plugged-in key's stub, for ssh.",
	Long: `Point a stable symlink (and its .pub sibling) at the resolved signing
key's stub, then print the link path. ssh can then name one fixed IdentityFile
that always follows whichever YubiKey is plugged in. With no argument the link
is ~/.ssh/id_sk_current; pass a path to place it elsewhere. Like git's callout
it never prompts; when several keys are connected, narrow with
--security-key/--username. Prefers the ed25519 key when a user has several types
enrolled. Drive it from ssh's Match exec so the right identity is selected on
connect:

  Match host github.com exec "dotty signing-key link >/dev/null"
      IdentityFile ~/.ssh/id_sk_current
      IdentitiesOnly yes
      IdentityAgent none`,
	Example: `  dotty signing-key link
  dotty signing-key link ~/.ssh/id_sk_work
  dotty signing-key link --security-key=work`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve non-interactively: a Match exec prompt would hang or go unseen.
		base := cli.System()
		ios := cli.IOStreams{In: nil, Out: base.Out, ErrOut: base.ErrOut}

		dataDir, err := cli.DataDir()
		if err != nil {
			return err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		store, err := securitykey.LoadStore(securitykey.StorePath(dataDir))
		if err != nil {
			return err
		}

		serial, err := securitykey.ResolveSerial(cmd.Context(), newRunner(ios), store, ios, signingKeyFlags.SecurityKey)
		if err != nil {
			return err
		}
		refs, err := signingkey.Scan(dataDir, []string{serial}, signingKeyFlags.Username)
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			return fmt.Errorf("%w for YubiKey %s (run `dotty signing-key new`)", signingkey.ErrKeyNotFound, serial)
		}
		// Deterministic choice: ed25519 (the default type) wins.
		sort.Slice(refs, func(i, j int) bool { return refs[i].Type > refs[j].Type })

		linkPath := signingkey.DefaultLinkPath(home)
		if len(args) == 1 {
			linkPath = args[0]
		}
		if err := signingkey.Link(linkPath, refs[0]); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(ios.Out, linkPath)
		return nil
	},
}

func init() {
	signingKeyCmd.AddCommand(signingKeyLinkCmd)
}
