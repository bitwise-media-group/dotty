// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env <verb>",
	Short: "Migrate legacy keychain credentials to fnox.",
	Long: `Secrets are managed by fnox (https://fnox.jdx.dev): fnox set / get / list /
remove replace the old dotty env verbs, fnox exec runs a command with them in
its environment, and fnox export writes a .env for tools that insist on one.
dotty installs fnox with its packages, activates its shell hook, and sets up
an age provider whose identity lives in the macOS Keychain.

The one verb left here carries credentials out of the old dotty env store —
keychain namespaces and .env.dotty templates — into fnox.`,
	Example: `  dotty env migrate
  dotty env migrate --namespace aws --purge
  fnox -P aws exec -- aws s3 ls`,
	// A noun without a Run prints help for any stray argument; the removed
	// verbs deserve a real error that names their fnox replacement.
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if replacement, ok := movedVerbs[args[0]]; ok {
			return fmt.Errorf("unknown command %q for \"dotty env\": it moved to fnox; use %s", args[0], replacement)
		}
		return fmt.Errorf("unknown command %q for \"dotty env\"", args[0])
	},
}

// movedVerbs maps each removed env verb to the fnox invocation that took
// its place.
var movedVerbs = map[string]string{
	"add":    "fnox set [-P <ns>] KEY",
	"get":    "fnox get [-P <ns>] KEY",
	"list":   "fnox list [-P <ns>]",
	"ls":     "fnox list [-P <ns>]",
	"remove": "fnox remove [-P <ns>] KEY",
	"rm":     "fnox remove [-P <ns>] KEY",
	"run":    "fnox exec [-P <ns>] -- <command>",
	"use":    "fnox export --all -o .env",
}

// defaultEnvFile is the project-local template the old env verbs fell back
// to; migrate picks it up from the working directory when no PATH is given.
const defaultEnvFile = ".env.dotty"

func init() {
	rootCmd.AddCommand(envCmd)
}
