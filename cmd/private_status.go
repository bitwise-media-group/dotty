// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var privateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report each private entry's freshness, plus repository hygiene.",
	Long: `Classify every entry the profile carries without touching hardware:
ok (everything agrees), stale (the repository changed; run dotty private
link), drifted (the decrypted copy was edited; run dotty private encrypt),
conflict (both changed; reconcile by hand), or missing (never decrypted
here). Repository hygiene findings — plaintext siblings, sensitive-looking
unencrypted files, half-finished enrollments — are reported for the whole
repository.`,
	Example: `  dotty private status
  dotty private status --profile work`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		repo, err := resolvePrivateRepo()
		if err != nil {
			return err
		}
		name, err := privateProfileName()
		if err != nil {
			return err
		}
		dataDir, err := cli.DataDir()
		if err != nil {
			return err
		}
		statuses, err := privdot.Status(dataDir, repo, name)
		if err != nil {
			return err
		}
		if len(statuses) == 0 {
			tui.Infof(ios, "Profile %s carries no private entries yet", name)
		}
		for _, s := range statuses {
			marker := " "
			if !s.Encrypted {
				marker = "p" // deployed as-is, no ciphertext
			}
			_, _ = fmt.Fprintf(ios.Out, "%-8s %s %s\n", s.State, marker, s.Rel)
		}
		findings, err := privdot.VerifyRepo(repo)
		if err != nil {
			return err
		}
		for _, f := range findings {
			tui.Warnf(ios, "%s: %s", f.Path, f.Problem)
		}
		return nil
	},
}

func init() {
	privateCmd.AddCommand(privateStatusCmd)
}
