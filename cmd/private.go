// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/privdot"
	"github.com/bitwise-media-group/dotty/internal/profile"
	"github.com/bitwise-media-group/dotty/internal/scaffold"
)

// PrivateFlags holds the flags shared by the private verbs.
type PrivateFlags struct {
	Repo string
}

var privateFlags = PrivateFlags{}

var privateCmd = &cobra.Command{
	Use:   "private <verb>",
	Short: "Manage the encrypted private dotfiles repository.",
	Long: `The private repository carries what the public one must not: git identities,
ssh host configuration, known_hosts — anything that leaks PII. Each profile
keeps its own home tree there, age-encrypted to that profile's security keys
alone, so the repository stores ciphertext only and a leak exposes nothing.
Decrypted files live under the dotty data directory (0700) and reach $HOME
through an active-profile symlink, so activating another profile swaps the
whole private identity at once.`,
	Example: `  dotty private init ~/Repos/dotfiles.private
  dotty private enroll --serial 17741369
  dotty private encrypt ~/.ssh/known_hosts
  dotty private status`,
}

func init() {
	privateCmd.PersistentFlags().StringVar(&privateFlags.Repo, "repo", "",
		"private repository (default: the active profile's stored answer)")
	rootCmd.AddCommand(privateCmd)
}

// resolvePrivateRepo locates the private repository the verbs operate on —
// --repo, the enclosing private repository, or the path the active profile's
// answers stored — and requires it to carry the private marker.
func resolvePrivateRepo() (string, error) {
	repo, err := privateRepoPath()
	if err != nil {
		return "", err
	}
	if !privdot.IsRepo(repo) {
		return "", fmt.Errorf("%s is not a private repository (no %s marker); run dotty private init",
			repo, scaffold.PrivateMarker)
	}
	return repo, nil
}

// privateRepoPath resolves the private repository path without requiring it
// to exist yet — init scaffolds the very marker resolvePrivateRepo demands.
func privateRepoPath() (string, error) {
	repo := privateFlags.Repo
	if repo == "" {
		repo = privdot.EnclosingRepo()
	}
	if repo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home: %w", err)
		}
		_, answers, err := activeProfileAnswers()
		if err != nil {
			return "", err
		}
		if repo = privateRepoFromAnswers(answers, home); repo == "" {
			return "", fmt.Errorf("no private repository configured; run dotty private init <path> first")
		}
	}
	return cli.ExpandHome(repo)
}

// privateRepoFromAnswers resolves a profile's stored private-repo path to an
// absolute one; "" when the profile keeps none.
func privateRepoFromAnswers(answers scaffold.Answers, home string) string {
	if answers.PrivateRepo == "" {
		return ""
	}
	repo := scaffold.ExpandTilde(answers.PrivateRepo, home)
	if !filepath.IsAbs(repo) {
		repo = filepath.Join(scaffold.ExpandTilde(answers.ReposDir, home), repo)
	}
	return repo
}

// activeProfileAnswers loads the active profile's directory and answers.
func activeProfileAnswers() (string, scaffold.Answers, error) {
	configDir, err := cli.ConfigDir()
	if err != nil {
		return "", scaffold.Answers{}, err
	}
	activeDir, err := profile.ActiveDir(configDir)
	if err != nil {
		return "", scaffold.Answers{}, fmt.Errorf("no active profile; run dotty init first: %w", err)
	}
	answers, err := scaffold.LoadAnswers(activeDir)
	if err != nil {
		return "", scaffold.Answers{}, fmt.Errorf(
			"active profile has no %s; run dotty init first: %w", scaffold.AnswersFile, err)
	}
	return activeDir, answers, nil
}

// privateProfileName resolves which private profile a verb operates on: the
// global --profile flag, else the active profile. Private profile names
// mirror the public repository's by convention.
func privateProfileName() (string, error) {
	configDir, err := cli.ConfigDir()
	if err != nil {
		return "", err
	}
	return profileName(configDir, nil)
}

// homeRel normalizes a user-supplied path — absolute, ~-prefixed, or already
// $HOME-relative — to the $HOME-relative form private entries are keyed by.
func homeRel(path, home string) (string, error) {
	expanded, err := cli.ExpandHome(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		return filepath.Clean(expanded), nil
	}
	rel, err := filepath.Rel(home, expanded)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the home directory; private entries are $HOME-relative", path)
	}
	return rel, nil
}
