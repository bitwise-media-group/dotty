// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tmux"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var tmuxNewCmd = &cobra.Command{
	Use:   "new [query]",
	Short: "Start (or attach to) a repository's tmux dev session.",
	Long: `Pick a repository and create-or-attach its tmux session: the editor on the
first window over a small shell split, one window per installed coding agent
(opencode, grok, codex, claude), and a shell window. The session is named after the
repository, so rerunning the command attaches to the existing session.

Repositories are discovered up to four levels deep under $REPOS_DIR (default
~/Repos). A query narrows the picklist and auto-selects a single match; the
query "." searches from the current directory instead, which resolves to the
enclosing repository.`,
	Example: `  dotty tmux new
  dotty tmux new dotty
  dotty tmux new .`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		var query string
		if len(args) > 0 {
			query = args[0]
		}
		dir, err := pickRepo(ios, query)
		if err != nil || dir == "" {
			return err
		}

		ctx := cmd.Context()
		runner := newRunner(ios)
		name := tmux.SessionName(dir)
		if !tmux.HasSession(ctx, runner, name) {
			editor, editorArgs := cli.EditorCommand()
			if err := tmux.NewSession(ctx, runner, name, dir, editor, editorArgs...); err != nil {
				return err
			}
		}

		// Title the tab (OSC 0) with the session, not the launcher invocation.
		_, _ = fmt.Fprintf(ios.Out, "\033]0;%s\007", name)
		return tmux.Attach(ctx, runner, name, dir, os.Getenv("TMUX") != "")
	},
}

func init() {
	tmuxCmd.AddCommand(tmuxNewCmd)
}

// pickRepo resolves the repository to open: the repos under $REPOS_DIR
// (default ~/Repos, falling back to the current directory when absent),
// narrowed by query. A single candidate is returned without prompting;
// several prompt with a fuzzy picklist. Backing out of the prompt returns ""
// and no error.
func pickRepo(ios cli.IOStreams, query string) (string, error) {
	root := os.Getenv("REPOS_DIR")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home: %w", err)
		}
		root = filepath.Join(home, "Repos")
	}
	if query == "." {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
		root, query = wd, ""
	} else if _, err := os.Stat(root); err != nil {
		root = "."
	}

	repos := tmux.FindRepos(root, 4)
	if query != "" {
		q := strings.ToLower(query)
		repos = slices.DeleteFunc(repos, func(p string) bool {
			return !strings.Contains(strings.ToLower(p), q)
		})
	}
	switch len(repos) {
	case 0:
		if query != "" {
			return "", fmt.Errorf("no repository matching %q under %s", query, root)
		}
		return "", fmt.Errorf("no repositories under %s", root)
	case 1:
		return repos[0], nil
	}

	options := make([]tui.Option, len(repos))
	for i, p := range repos {
		label := p
		if rel, err := filepath.Rel(root, p); err == nil {
			label = rel
		}
		options[i] = tui.Option{Label: label, Value: p}
	}
	selected, err := tui.FuzzySelect(ios, "Start a session for which repository?", options)
	if errors.Is(err, tui.ErrAborted) {
		return "", nil // esc backs out without starting anything
	}
	if errors.Is(err, tui.ErrNotInteractive) {
		return "", errors.New("several repositories match; pass a query that narrows to one")
	}
	return selected, err
}
