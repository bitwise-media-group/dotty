// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
	"github.com/bitwise-media-group/dotty/internal/worktree"
)

var gitWorktreeStartCmd = &cobra.Command{
	Use:   "start [repo] [suffix]",
	Short: "Create or reuse an agent/* worktree and print its path.",
	Long: `Create (or reuse) a worktree for repo on an agent/<repo>-<suffix> branch at
the configured worktree location, and print its path — the path is the
command's result, so hooks and scripts capture stdout.

repo falls back to $CLAUDE_PROJECT_DIR, then the enclosing repository.
suffix falls back to the WorktreeCreate hook JSON on stdin ({"name": ...}),
then a UTC timestamp.`,
	Example: `  dotty git worktree start
  dotty git worktree start ~/Repos/dotty fix-linker
  echo '{"name":"fix-1"}' | dotty git worktree start   # hook form`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		ctx := cmd.Context()
		runner := newRunner(ios)

		repo := argAt(args, 0)
		if repo == "" {
			repo = os.Getenv("CLAUDE_PROJECT_DIR")
		}
		if repo == "" {
			if out, err := runner.Output(ctx, "git", "rev-parse", "--show-toplevel"); err == nil {
				repo = strings.TrimSpace(string(out))
			}
		}
		if repo == "" {
			return errors.New("no repository found; pass one as the first argument")
		}
		repo, err := cli.ExpandHome(repo)
		if err != nil {
			return err
		}

		suffix := argAt(args, 1)
		if suffix == "" {
			if data, ok := readIfPiped(cmd.InOrStdin()); ok {
				suffix = worktree.ParseStartName(data)
			}
		}
		if suffix == "" {
			suffix = time.Now().UTC().Format("20060102-150405")
			tui.Warnf(ios, "No suffix given; using timestamp %s", suffix)
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		names := worktree.Derive(repo, suffix, worktree.Root(repo, worktreesSetting(), home))

		if err := startWorktree(ctx, ios, runner, repo, names); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(ios.Out, names.Path)
		return nil
	},
}

func init() {
	gitWorktreeCmd.AddCommand(gitWorktreeStartCmd)
}

// startWorktree makes names.Path exist: reused when already there, checked
// out when the branch survives from an earlier run, created fresh otherwise.
func startWorktree(ctx context.Context, ios cli.IOStreams, r *cli.ExecRunner, repo string, names worktree.Names) error {
	if info, err := os.Stat(names.Path); err == nil && info.IsDir() {
		if err := checkReusable(ctx, r, repo, names.Path); err != nil {
			return err
		}
		tui.Warnf(ios, "Worktree already exists at %s; reusing", names.Path)
		return nil
	}
	_, refErr := r.Output(ctx, "git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/"+names.Branch)
	if refErr == nil {
		tui.Infof(ios, "Branch %s exists; checking it out at %s", names.Branch, names.Path)
		if _, err := r.Output(ctx, "git", "-C", repo, "worktree", "add", names.Path, names.Branch); err != nil {
			return fmt.Errorf("check out worktree: %w", err)
		}
		return nil
	}
	tui.Infof(ios, "Creating worktree %s", names.Name)
	if _, err := r.Output(ctx, "git", "-C", repo, "worktree", "add", "-b", names.Branch, names.Path); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}
	return nil
}

// checkReusable confirms the existing directory at path is a worktree of
// repo. A shared worktree root holds every repository's worktrees, so a
// same-named one may belong to another repository with the same basename —
// handing that back would point the agent at the wrong code.
func checkReusable(ctx context.Context, r *cli.ExecRunner, repo, path string) error {
	notOurs := fmt.Errorf("%s exists but is not a worktree of %s; remove it or choose another name", path, repo)
	top, err := r.Output(ctx, "git", "-C", path, "rev-parse", "--show-toplevel")
	if err != nil || !samePath(strings.TrimSpace(string(top)), path) {
		return notOurs
	}
	want, err := git.CommonDir(ctx, r, repo)
	if err != nil {
		return err
	}
	if got, err := git.CommonDir(ctx, r, path); err != nil || got != want {
		return notOurs
	}
	return nil
}

// samePath reports whether a and b name the same location once symlinks
// are resolved.
func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// argAt returns args[i] or "".
func argAt(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}
