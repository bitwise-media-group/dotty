// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

var gitDoneCmd = &cobra.Command{
	Use:   "done",
	Short: "Return to trunk, prune merged branches everywhere, fast-forward.",
	Long: `Finish a piece of work and reset to a clean, current trunk:

  1. Check out the trunk branch (main)
  2. Fetch all remotes with --prune
  3. Remove the worktrees of local branches already merged into trunk
     (prompting if a worktree has uncommitted changes)
  4. Delete every local branch already merged into trunk (upstream/main when
     present, else origin/main), dropping it from any recorded stack
  5. Delete every origin branch already merged into trunk
  6. Fast-forward the local trunk branch to the remote
  7. Push the trunk branch to origin (keeps a fork's main in step)

Step 3 is what collects agent worktrees whose removal hook never fired. A
worktree holding uncommitted changes is kept, along with its branch, unless
you confirm on a terminal; so is the worktree you are standing in.

Any remaining stack that has diverged from trunk is reported; check it out
and run ` + "`dotty git sync`" + ` to rebase and re-sign it.`,
	Example: `  dotty git done`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ios := cli.System()
		r := newRunner(ios)
		ctx := cmd.Context()

		trunk, err := git.ResolveTrunk(ctx, r)
		if err != nil {
			return err
		}
		if err := git.Checkout(ctx, r, trunk.Branch); err != nil {
			return err
		}
		if err := git.FetchAllPrune(ctx, r); err != nil {
			tui.Warnf(ios, "Fetch: %v", err)
		}

		locals, err := git.MergedLocalBranches(ctx, r, trunk)
		if err != nil {
			return err
		}
		kept := removeMergedWorktrees(ctx, ios, r, locals)
		for _, b := range locals {
			if kept[b] {
				continue
			}
			if s, err := git.LoadStack(ctx, r, b); err == nil {
				if _, err := git.RemoveLayer(ctx, r, s, b); err != nil {
					tui.Warnf(ios, "Drop %s from stack %s: %v", b, s.ID, err)
				}
			} else if !errors.Is(err, git.ErrNotInStack) {
				tui.Warnf(ios, "%v", err)
			}
			if err := git.DeleteBranchLocal(ctx, r, b); err != nil {
				tui.Warnf(ios, "%v", err)
				continue
			}
			tui.Infof(ios, "Deleted %s", b)
		}

		remote, remoteErr := git.PushRemote(ctx, r)
		if remoteErr == nil {
			remotes, err := git.MergedRemoteBranches(ctx, r, remote, trunk)
			if err != nil {
				return err
			}
			for _, b := range remotes {
				if err := git.DeleteBranchRemote(ctx, r, remote, b); err != nil {
					tui.Warnf(ios, "%v", err)
					continue
				}
				tui.Infof(ios, "Deleted %s/%s", remote, b)
			}
		}

		if err := git.FastForward(ctx, r, trunk.Ref()); err != nil {
			return err
		}
		if remoteErr == nil {
			if err := git.PushTrunk(ctx, r, remote, trunk); err != nil {
				tui.Warnf(ios, "%v", err)
			} else {
				tui.Infof(ios, "Pushed %s to %s", trunk.Branch, remote)
			}
		}

		stacks, err := git.ListStacks(ctx, r)
		if err != nil {
			tui.Warnf(ios, "%v", err)
		}
		for _, s := range stacks {
			rows := git.Status(ctx, r, s, trunk, "")
			if git.AnyDiverged(rows) || git.AnyStale(ctx, r, rows) {
				tui.Warnf(ios, "Stack %s (%s) needs a rebase; check it out and run dotty git sync",
					s.ID, s.Bottom())
			}
		}

		tui.Successf(ios, "On %s, up to date with %s", trunk.Branch, trunk.Ref())
		return nil
	},
}

func init() {
	gitCmd.AddCommand(gitDoneCmd)
}

// removeMergedWorktrees removes the linked worktrees of the merged branches
// and returns the branches whose worktrees stay — git refuses to delete a
// branch still checked out, so the caller skips them. Unpushed commits do not
// matter here: a merged branch's commits are already on trunk.
func removeMergedWorktrees(ctx context.Context, ios cli.IOStreams, r *cli.ExecRunner, merged []string) map[string]bool {
	kept := map[string]bool{}
	cwd, err := os.Getwd()
	if err != nil {
		tui.Warnf(ios, "Skipping worktree cleanup: %v", err)
		return kept
	}
	if err := git.PruneWorktrees(ctx, r, cwd); err != nil {
		tui.Warnf(ios, "%v", err)
	}
	trees, err := git.ListWorktrees(ctx, r, cwd)
	if err != nil {
		tui.Warnf(ios, "Skipping worktree cleanup: %v", err)
		return kept
	}
	byBranch := map[string]git.Worktree{}
	for _, wt := range trees {
		if !wt.Main && wt.Branch != "" {
			byBranch[wt.Branch] = wt
		}
	}

	for _, b := range merged {
		wt, ok := byBranch[b]
		if !ok {
			continue
		}
		if pathContains(wt.Path, cwd) {
			tui.Warnf(ios, "Keeping %s and its worktree %s: you are inside it", b, wt.Path)
			kept[b] = true
			continue
		}
		state, err := git.InspectWorktree(ctx, r, wt.Path)
		if err != nil {
			tui.Warnf(ios, "Keeping %s: %v", b, err)
			kept[b] = true
			continue
		}
		if state.Uncommitted > 0 {
			tui.Warnf(ios, "Worktree %s [%s] has %d uncommitted change(s)", wt.Path, b, state.Uncommitted)
			ok, err := tui.Confirm(ios, "Remove worktree anyway?", "Its uncommitted changes will be lost.")
			if err != nil || !ok {
				tui.Warnf(ios, "Keeping %s and its worktree %s", b, wt.Path)
				kept[b] = true
				continue
			}
		}
		if err := removeWorktree(ctx, ios, r, wt.Path, removeOptions{yes: true, keepBranch: true}); err != nil {
			tui.Warnf(ios, "Keeping %s: %v", b, err)
			kept[b] = true
		}
	}
	return kept
}

// pathContains reports whether path is dir or lies beneath it, comparing
// symlink-resolved paths.
func pathContains(dir, path string) bool {
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	if p, err := filepath.EvalSymlinks(path); err == nil {
		path = p
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}
