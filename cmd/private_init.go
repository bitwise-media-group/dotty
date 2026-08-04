// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/scaffold"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var privateInitCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Scaffold or adopt the private repository.",
	Long: `Create the private repository skeleton at path — the private marker, the
git attributes and ignore guards, a pre-commit hook running dotty private
verify, and an empty profile matching the active one — and record the path in
the active profile's answers so every machine of the class finds it. An
existing repository is adopted: nothing already there is touched, so
re-running init is always safe. Without a path, the stored answer is reused,
falling back to dotfiles.private beside the public repository.`,
	Example: `  dotty private init ~/Repos/dotfiles.private
  dotty private init`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		ctx := cmd.Context()
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home: %w", err)
		}
		activeDir, answers, err := activeProfileAnswers()
		if err != nil {
			return err
		}
		reposDir := scaffold.ExpandTilde(answers.ReposDir, home)

		repo := ""
		switch {
		case len(args) == 1:
			repo = args[0]
		case privateFlags.Repo != "":
			repo = privateFlags.Repo
		case answers.PrivateRepo != "":
			if repo = scaffold.ExpandTilde(answers.PrivateRepo, home); !filepath.IsAbs(repo) {
				repo = filepath.Join(reposDir, repo)
			}
		default:
			repo = filepath.Join(reposDir, "dotfiles.private")
		}
		if repo, err = cli.ExpandHome(repo); err != nil {
			return err
		}
		if !filepath.IsAbs(repo) {
			if repo, err = filepath.Abs(repo); err != nil {
				return fmt.Errorf("resolve %s: %w", args[0], err)
			}
		}

		name, err := privateProfileName()
		if err != nil {
			return err
		}
		if err := privdot.Scaffold(repo, name); err != nil {
			return err
		}
		runner := newRunner(ios)
		if err := git.InitRepo(ctx, runner, repo); err != nil {
			return err
		}
		// The hook only guards when git looks for it in the committed
		// .githooks; the default hooks dir is not part of the repository.
		if _, err := runner.Output(ctx, "git", "-C", repo, "config", "core.hooksPath", ".githooks"); err != nil {
			return err
		}

		// Store the path portably, like Repo: relative to ReposDir when
		// inside it, tilde-folded otherwise.
		stored := scaffold.TildePath(repo, home)
		if rel, err := filepath.Rel(reposDir, repo); err == nil && filepath.IsLocal(rel) {
			stored = rel
		}
		if answers.PrivateRepo != stored {
			answers.PrivateRepo = stored
			if err := scaffold.SaveAnswers(activeDir, answers); err != nil {
				return err
			}
			tui.Successf(ios, "Recorded private repository %s in profile %s", stored, answers.ProfileName)
		}
		tui.Successf(ios, "Private repository ready at %s (profile %s)", repo, name)
		tui.Infof(ios, "Next: dotty private enroll to add a security key, then dotty private encrypt <file>")
		return nil
	},
}

func init() {
	privateCmd.AddCommand(privateInitCmd)
}
