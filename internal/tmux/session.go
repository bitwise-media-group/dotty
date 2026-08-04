// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tmux

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// SessionRunner runs tmux for session management. Output captures the pane
// and window ids the layout steps target, LookPath probes for installed
// agents, and RunInteractive hands the terminal over for attach.
type SessionRunner interface {
	Runner
	LookPath(name string) (string, error)
	RunInteractive(ctx context.Context, name string, args ...string) error
}

// agentWindows lists the coding agents that get a window when installed.
// Each window is inserted directly after the editor window, so the final
// order is the reverse of this one (with the shell window first). The window
// glyphs are lobe-icons brand codepoints (U+F4000–U+F47FF), which render only
// with the lobe-icons font installed and mapped in the terminal — both wired
// up by `dotty init` (~/Library/Fonts/lobe-icons.ttf + ghostty's
// font-codepoint-map); the pinned release lives in internal/fonts.
// codex runs with --profile dotty so it loads dotty's rendered profile layer
// ($CODEX_HOME/dotty.config.toml) — config.toml is machine-local and carries
// no dotty settings.
var agentWindows = []struct {
	bin, window string
	args        []string
}{
	{"opencode", "\U000F40D3  opencode", nil},
	{"grok", "\U000F4079  grok", nil},
	{"codex", "\U000F403E  codex", []string{"--profile", "dotty"}},
	{"claude", "\U000F4036  claude", nil},
}

// SessionName derives the tmux session name from a repository path: its
// basename with dots replaced, since '.' separates window and pane in tmux
// target syntax.
func SessionName(dir string) string {
	return strings.ReplaceAll(filepath.Base(dir), ".", "_")
}

// HasSession reports whether a session named name is running.
func HasSession(ctx context.Context, r Runner, name string) bool {
	_, err := r.Output(ctx, "tmux", "has-session", "-t", name)
	return err == nil
}

// NewSession creates the detached dev-session layout rooted at dir: the
// editor on the first window over a small shell split, one window per
// installed coding agent, and a shell window. Only the two calls the layout
// depends on can fail it; a missed split or window degrades the session
// rather than aborting it.
func NewSession(ctx context.Context, r SessionRunner, name, dir, editor string, editorArgs ...string) error {
	argv := slices.Concat(
		[]string{
			"-u", "new-session", "-d", "-P", "-F", "#{pane_id}",
			"-s", name, "-n", "  " + filepath.Base(editor), "-c", dir, "-x", "-", "-y", "-", editor,
		},
		editorArgs,
		[]string{"."},
	)
	pane, err := r.Output(ctx, "tmux", argv...)
	if err != nil {
		return fmt.Errorf("create session %s: %w", name, err)
	}
	editorPane := strings.TrimSpace(string(pane))

	win, err := r.Output(ctx, "tmux", "display-message", "-p", "-t", editorPane, "#{window_id}")
	if err != nil {
		return fmt.Errorf("locate editor window: %w", err)
	}
	window := strings.TrimSpace(string(win))

	_, _ = r.Output(ctx, "tmux", "split-window", "-t", editorPane, "-v", "-l", "10%", "-c", dir)
	_, _ = r.Output(ctx, "tmux", "select-pane", "-t", editorPane)
	for _, a := range agentWindows {
		path, err := r.LookPath(a.bin)
		if err != nil {
			continue // agent not installed
		}
		argv := append([]string{"new-window", "-a", "-d", "-t", window, "-c", dir, "-n", a.window, path}, a.args...)
		_, _ = r.Output(ctx, "tmux", argv...)
	}
	_, _ = r.Output(ctx, "tmux", "new-window", "-a", "-d", "-t", window, "-n", "  zsh", "-c", dir)
	return nil
}

// Attach hands the terminal to the named session: switch-client when already
// inside tmux (attaching would nest sessions), attach-session otherwise.
func Attach(ctx context.Context, r SessionRunner, name, dir string, insideTmux bool) error {
	if insideTmux {
		return r.RunInteractive(ctx, "tmux", "switch-client", "-t", name)
	}
	return r.RunInteractive(ctx, "tmux", "-u", "attach-session", "-t", name, "-c", dir)
}

// FindRepos walks root at most maxDepth levels deep and returns every
// directory containing a .git entry (directory or worktree file), without
// descending into matches.
func FindRepos(root string, maxDepth int) []string {
	var repos []string
	base := depth(root)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
			repos = append(repos, path)
			return fs.SkipDir
		}
		if depth(path)-base >= maxDepth {
			return fs.SkipDir
		}
		return nil
	})
	return repos
}

func depth(p string) int { return strings.Count(filepath.Clean(p), string(filepath.Separator)) }
