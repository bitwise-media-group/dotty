// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/mise"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/scaffold"
)

// DotfilesFlags holds the flags shared by the dotfiles verbs.
type DotfilesFlags struct {
	Repo string
}

var dotfilesFlags = DotfilesFlags{}

var dotfilesCmd = &cobra.Command{
	Use:   "dotfiles",
	Short: "Operate on the dotfiles repository dotty init generated.",
	Long: `Manage dotfiles through mise against the selected profile's mise config.
link registers the repository's home tree, applies symlinks by default and
regular copies for Grok, and tracks repository sources in mise history.
All mise dot commands and flags are available here. Use add --changed to
capture live copy edits back into the repository before applying it again.
restore with paths proxies rollback; --timestamp restores legacy dotty backups.`,
	Example: `  dotty dotfiles status
  dotty dotfiles link --on-conflict=backup
  dotty dotfiles add --changed
  dotty dotfiles track ~/.zshrc
  dotty dotfiles rollback ~/.zshrc`,
	Args: cobra.ArbitraryArgs,
	RunE: proxyDotfiles,
}

func init() {
	dotfilesCmd.PersistentFlags().StringVar(&dotfilesFlags.Repo, "repo", "",
		"dotfiles repository (default: found via $REPOS_DIR)")
	rootCmd.AddCommand(dotfilesCmd)
	for _, verb := range []string{
		"add", "apply", "capture", "conflicts", "diff", "edit", "exclude", "history", "include", "origin", "paths",
		"pull", "recover", "rollback", "save", "sync", "track", "unapply", "undo", "untrack", "watch", "help",
	} {
		proxy := &cobra.Command{
			Use:                verb + " [mise arguments]",
			Short:              "Proxy mise dot " + verb + " against the selected profile.",
			Long:               "Forward arguments and flags to mise dot " + verb + ". Run with --help for mise's command help.",
			Example:            "  dotty dotfiles " + verb + " --help",
			DisableFlagParsing: true,
			RunE: func(cmd *cobra.Command, args []string) error {
				return proxyDotfiles(cmd, append([]string{cmd.Name()}, args...))
			},
		}
		if verb == "unapply" {
			proxy.Aliases = []string{"remove"}
		}
		dotfilesCmd.AddCommand(proxy)
	}
}

func proxyDotfiles(cmd *cobra.Command, args []string) error {
	// Help belongs to mise for proxied verbs; keep it in the raw argv.
	own, rest, help := cli.ExtractFlags(args, map[string]bool{"repo": true})
	if help {
		if len(rest) == 0 {
			return cmd.Help()
		}
		rest = append(rest, "--help")
	}
	if repo, ok := own["repo"]; ok {
		dotfilesFlags.Repo = repo
	}
	client, err := newDotfilesClient(cmd.Context(), cli.System())
	if err != nil {
		return err
	}
	return client.Dotfiles(cmd.Context(), rest...)
}

func newDotfilesClient(ctx context.Context, ios cli.IOStreams) (*mise.Client, error) {
	if dotfilesFlags.Repo == "" {
		return newMiseClient(ctx, ios)
	}
	repo, answers, err := resolveDotfilesRepo()
	if err != nil {
		return nil, err
	}
	client, err := newMiseClient(ctx, ios)
	if err != nil {
		return nil, err
	}
	client.Dir = mise.Dir(filepath.Join(scaffold.ProfilesDir(repo), answers.ProfileName))
	if err := mise.EnsureConfig(client.Dir); err != nil {
		return nil, err
	}
	return client, nil
}

// resolveDotfilesRepo locates the repository the verbs operate on — --repo,
// the enclosing dotfiles repository, or the paths the active profile's
// answers stored — and loads those answers, which own everything that
// varies across machine classes.
func resolveDotfilesRepo() (string, scaffold.Answers, error) {
	configDir, err := cli.ConfigDir()
	if err != nil {
		return "", scaffold.Answers{}, err
	}
	activeDir := profile.Dir(configDir, rootFlags.Profile)
	if rootFlags.Profile == "" {
		activeDir, err = profile.ActiveDir(configDir)
	} else if !profile.Exists(configDir, rootFlags.Profile) {
		err = profile.ErrNotFound
	}
	if err != nil {
		return "", scaffold.Answers{}, fmt.Errorf("no active profile; run dotty init first: %w", err)
	}
	answers, err := scaffold.LoadAnswers(activeDir)
	if err != nil {
		return "", scaffold.Answers{}, fmt.Errorf(
			"active profile has no %s; run dotty init first: %w", scaffold.AnswersFile, err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", scaffold.Answers{}, fmt.Errorf("resolve home: %w", err)
	}
	repo := dotfilesFlags.Repo
	if repo == "" {
		repo = scaffold.EnclosingRepo()
	}
	if repo == "" {
		reposDir := scaffold.ExpandTilde(answers.ReposDir, home)
		if repo = scaffold.ExpandTilde(answers.Repo, home); repo == "" {
			repo = filepath.Join(reposDir, "dotfiles")
		} else if !filepath.IsAbs(repo) {
			repo = filepath.Join(reposDir, repo)
		}
	}
	if repo, err = cli.ExpandHome(repo); err != nil {
		return "", scaffold.Answers{}, err
	}
	if _, err := os.Stat(scaffold.HomeDir(repo)); err != nil {
		if _, legacyErr := os.Stat(filepath.Join(repo, "stow")); legacyErr == nil {
			return "", scaffold.Answers{},
				fmt.Errorf("%s uses the legacy layout; run dotty init to migrate it first", repo)
		}
		return "", scaffold.Answers{},
			fmt.Errorf("%s is not a dotfiles repository (no home tree); pass --repo: %w", repo, err)
	}
	return repo, answers, nil
}
