// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// PrivateVerifyFlags holds the flags for `dotty private verify`.
type PrivateVerifyFlags struct {
	Staged bool
}

var privateVerifyFlags = PrivateVerifyFlags{}

var privateVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check the repository for plaintext accidents.",
	Long: `Scan for hygiene problems that must never be committed: a plaintext file
sitting beside its ciphertext, a sensitive-looking file that was never
encrypted, or a profile whose enrollment is half-finished. With --staged only
the files staged in git are checked — the mode the scaffolded pre-commit hook
runs — and content is never read, so the check is safe anywhere. Any finding
exits non-zero.`,
	Example: `  dotty private verify
  dotty private verify --staged`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		repo, err := resolvePrivateRepo()
		if err != nil {
			return err
		}
		var findings []privdot.Finding
		if privateVerifyFlags.Staged {
			out, err := newRunner(ios).Output(cmd.Context(), "git", "-C", repo,
				"diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z")
			if err != nil {
				return fmt.Errorf("list staged files: %w", err)
			}
			var rels []string
			for rel := range strings.SplitSeq(strings.TrimRight(string(out), "\x00"), "\x00") {
				if rel != "" {
					rels = append(rels, rel)
				}
			}
			findings = privdot.VerifyPaths(repo, rels)
		} else {
			if findings, err = privdot.VerifyRepo(repo); err != nil {
				return err
			}
		}
		for _, f := range findings {
			tui.Warnf(ios, "%s: %s", f.Path, f.Problem)
		}
		if len(findings) > 0 {
			return fmt.Errorf("%d finding(s); nothing was committed", len(findings))
		}
		return nil
	},
}

func init() {
	privateVerifyCmd.Flags().BoolVar(&privateVerifyFlags.Staged, "staged", false,
		"check only the files staged in git (pre-commit mode)")
	privateCmd.AddCommand(privateVerifyCmd)
}
