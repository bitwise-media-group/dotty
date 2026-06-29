// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// GitResignFlags holds the resign verb's flags. AmendHead is hidden: it is the
// rebase --exec re-entry point, not a user-facing option.
type GitResignFlags struct {
	Root        bool
	ResetAuthor bool
	Yes         bool
	AmendHead   bool
}

var gitResignFlags = GitResignFlags{}

var gitResignCmd = &cobra.Command{
	Use:   "resign [--root | <commitish>] [--reset-author]",
	Short: "Rebase and re-sign commits up to a commitish.",
	Long: `Rebase the commits up to a target and re-sign each one with your hardware
signing key. Pass --root to resign every commit from the start of history, or a
<commitish> to resign the commits after it (<commitish>..HEAD).

With --reset-author, each commit's author is also reset to your current
user.name/user.email, and any trailer naming the original author (for example
Co-authored-by: or Signed-off-by:) is rewritten to the new identity.

Resigning rewrites history: commits get new SHAs. It prompts for confirmation
unless --yes is given. Signing must already be configured — see
` + "`dotty signing-key sign --print-git-config`" + `.`,
	Example: `  dotty git resign HEAD~3
  dotty git resign --root --reset-author
  dotty git resign HEAD~5 --yes`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		r := newRunner(ios)

		// Re-entry from the rebase --exec step; amend the current commit.
		if gitResignFlags.AmendHead {
			return git.AmendHead(cmd.Context(), r)
		}

		hasBase := len(args) == 1
		switch {
		case gitResignFlags.Root && hasBase:
			return errors.New("--root and a <commitish> are mutually exclusive")
		case !gitResignFlags.Root && !hasBase:
			return errors.New("specify --root or a <commitish> to resign up to")
		}

		opts := git.Options{Root: gitResignFlags.Root, ResetAuthor: gitResignFlags.ResetAuthor}
		if hasBase {
			opts.Base = args[0]
		}

		if gitResignFlags.ResetAuthor {
			if err := git.EnsureIdentity(cmd.Context(), r); err != nil {
				return err
			}
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate the dotty binary: %w", err)
			}
			opts.Exe = exe
		}

		if !gitResignFlags.Yes {
			n, err := git.CommitCount(cmd.Context(), r, opts)
			if err != nil {
				return err
			}
			ok, err := tui.Confirm(ios,
				fmt.Sprintf("Rewrite and re-sign %d commit(s)?", n),
				"This rewrites history: commits get new SHAs.")
			if err != nil {
				if errors.Is(err, tui.ErrNotInteractive) {
					return fmt.Errorf("%w (pass --yes to skip confirmation)", err)
				}
				return err
			}
			if !ok {
				return nil
			}
		}

		return git.Resign(cmd.Context(), r, opts)
	},
}

func init() {
	gitResignCmd.Flags().BoolVar(&gitResignFlags.Root, "root", false,
		"resign every commit from the start of history")
	gitResignCmd.Flags().BoolVar(&gitResignFlags.ResetAuthor, "reset-author", false,
		"reset author to user.name/user.email and rewrite matching trailers")
	gitResignCmd.Flags().BoolVarP(&gitResignFlags.Yes, "yes", "y", false,
		"skip the confirmation prompt")
	gitResignCmd.Flags().BoolVar(&gitResignFlags.AmendHead, "amend-head", false,
		"internal: amend the current commit during rebase --exec")
	_ = gitResignCmd.Flags().MarkHidden("amend-head")
	gitCmd.AddCommand(gitResignCmd)
}
