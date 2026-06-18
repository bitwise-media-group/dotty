// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/env"
)

// EnvFlags holds the flags shared by the env verbs.
type EnvFlags struct {
	// Namespace is persistent on the noun so it parses in both positions:
	// `dotty env --namespace=aws add KEY` and `dotty env add --namespace=aws KEY`.
	Namespace string
}

var envFlags = EnvFlags{}

var envCmd = &cobra.Command{
	Use:   "env <verb>",
	Short: "Store and inject credentials from the macOS Keychain.",
	Long: `Manage generic credentials in the macOS login keychain and inject them into
templates and processes — the way the 1Password CLI does, but with no external
service. Secrets are grouped into namespaces; each namespace is a single
keychain item under the service name "dotty:<namespace>". get reads one value
(like op read), use fills a template (like op inject), and run launches a
process with the namespace's secrets in its environment (like op run).`,
	Example: `  dotty env add --namespace=aws AWS_ACCESS_KEY_ID
  dotty env list --namespace=aws
  dotty env get --namespace=aws AWS_ACCESS_KEY_ID
  dotty env run --namespace=aws -- aws s3 ls`,
}

// newEnvStore builds the credential store backed by the platform keychain.
func newEnvStore(ios cli.IOStreams) *env.Store {
	return env.NewStore(env.NewKeychain(newRunner(ios)))
}

func init() {
	envCmd.PersistentFlags().StringVar(&envFlags.Namespace, "namespace", "default",
		"credential namespace to operate on")
	rootCmd.AddCommand(envCmd)
}
