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

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/git"
	"github.com/bitwise-media-group/dotty/internal/tui"
	"github.com/bitwise-media-group/dotty/internal/worktree"
)

var gitWorktreeRemoveFlags struct {
	Yes bool
}

var gitWorktreeRemoveCmd = &cobra.Command{
	Use:     "remove [path]",
	Aliases: []string{"end"},
	Short:   "Remove a worktree, its agent/* branch, and its tmux session.",
	Long: `Tear a linked worktree down: kill the tmux session named after it, remove
the worktree from its repository, and delete its branch — but only an
agent/* branch, so a manually checked-out branch survives. A repository's
main checkout is never removed, and a worktree that is already gone exits
quietly.

Removal is forced past whatever git would normally refuse: populated
submodules, locks, and uncommitted changes. Work that would be lost —
uncommitted changes, or commits found on no remote and no other local branch
— is reported first and needs confirming: a prompt (defaulting to No) on a
terminal, --yes otherwise.

path falls back to the WorktreeRemove hook JSON on stdin
({"worktree_path": ...}), then the linked worktree enclosing the current
directory.`,
	Example: `  dotty git worktree remove ~/.local/share/dotty/worktrees/dotty-fix-1
  dotty git worktree remove --yes        # the worktree you are in
  echo '{"worktree_path":"..."}' | dotty git worktree remove --yes   # hook form`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ios := cli.System()
		ctx := cmd.Context()
		r := newRunner(ios)

		path := argAt(args, 0)
		if path == "" {
			if data, ok := readIfPiped(cmd.InOrStdin()); ok {
				path = worktree.ParseEndPath(data)
			}
		}
		if path == "" {
			path = enclosingLinkedWorktree(ctx, r)
		}
		if path == "" {
			return errors.New(`no worktree path; pass it as an argument, pipe {"worktree_path": ...}, ` +
				"or run inside a linked worktree")
		}
		path, err := cli.ExpandHome(path)
		if err != nil {
			return err
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", path, err)
		}
		return removeWorktree(ctx, ios, r, path, removeOptions{yes: gitWorktreeRemoveFlags.Yes})
	},
}

func init() {
	f := gitWorktreeRemoveCmd.Flags()
	f.BoolVarP(&gitWorktreeRemoveFlags.Yes, "yes", "y", false,
		"remove without confirming uncommitted or unpushed work")
	f.BoolVarP(&gitWorktreeRemoveFlags.Yes, "force", "f", false, "alias for --yes")
	excludeGitConfigFlags(f, "yes", "force")
	gitWorktreeCmd.AddCommand(gitWorktreeRemoveCmd)
}

// enclosingLinkedWorktree returns the top of the linked worktree containing
// the current directory, or "" when there is none.
func enclosingLinkedWorktree(ctx context.Context, r *cli.ExecRunner) string {
	out, err := r.Output(ctx, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	top := strings.TrimSpace(string(out))
	if linked, err := git.IsLinkedWorktree(ctx, r, top); err != nil || !linked {
		return ""
	}
	return top
}

// removeOptions tunes removeWorktree for its callers.
type removeOptions struct {
	// yes removes without confirming work that would be lost.
	yes bool
	// keepBranch leaves the branch for the caller to delete (git done drops
	// it from its stack first).
	keepBranch bool
}

// removeWorktree removes the linked worktree at path, its tmux session, and
// its agent/* branch. Lost work needs opts.yes or an interactive yes; a
// refusal is an error, so a hook caller sees the removal fail and keeps the
// worktree.
func removeWorktree(ctx context.Context, ios cli.IOStreams, r *cli.ExecRunner, path string, opts removeOptions) error {
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		tui.Warnf(ios, "Worktree at %s no longer exists", path)
		return nil
	}
	linked, err := git.IsLinkedWorktree(ctx, r, path)
	if err != nil {
		return err
	}
	if !linked {
		return fmt.Errorf("%s is a repository's main checkout, not a linked worktree; refusing to remove it", path)
	}
	repo, err := git.MainRepo(ctx, r, path)
	if err != nil {
		return err
	}

	var branch string
	if out, err := r.Output(ctx, "git", "-C", path, "branch", "--show-current"); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	label := branch
	if label == "" {
		label = "detached"
	}

	state, err := git.InspectWorktree(ctx, r, path)
	if err != nil {
		return err
	}
	if state.Dirty() {
		tui.Warnf(ios, "%s [%s] — uncommitted: %d, unpushed: %d", path, label, state.Uncommitted, state.Unpushed)
		if !opts.yes {
			if err := confirmRemoval(ios); err != nil {
				return err
			}
		}
	}

	session := filepath.Base(path)
	if _, err := r.LookPath("tmux"); err == nil && !inTmuxSession(ctx, r, session) {
		_, _ = r.Output(ctx, "tmux", "kill-session", "-t", "="+session)
	}

	if err := git.RemoveWorktree(ctx, r, repo, path); err != nil {
		return err
	}
	if !opts.keepBranch && worktree.IsAgentBranch(branch) {
		if _, err := r.Output(ctx, "git", "-C", repo, "branch", "-D", branch); err != nil {
			tui.Warnf(ios, "Delete branch %s: %v", branch, err)
		}
	}

	tui.Successf(ios, "Removed %s", session)
	return nil
}

// confirmRemoval asks before discarding work, defaulting to No. Without a
// terminal it errors, pointing at --yes.
func confirmRemoval(ios cli.IOStreams) error {
	refused := errors.New("worktree has uncommitted/unpushed work; pass --yes to remove it anyway")
	if !ios.IsInteractive() {
		return refused
	}
	ok, err := tui.Confirm(ios, "Remove worktree anyway?", "Uncommitted and unpushed work will be lost.")
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("worktree kept")
	}
	return nil
}

// inTmuxSession reports whether this process runs inside the tmux session
// named session — killing it would kill the hook's own parent.
func inTmuxSession(ctx context.Context, r *cli.ExecRunner, session string) bool {
	if os.Getenv("TMUX") == "" {
		return false
	}
	out, err := r.Output(ctx, "tmux", "display-message", "-p", "#S")
	return err == nil && strings.TrimSpace(string(out)) == session
}
