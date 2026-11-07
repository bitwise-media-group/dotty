// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeSessionRunner scripts Output responses by tmux subcommand and records
// every call so tests can assert the exact layout sequence.
type fakeSessionRunner struct {
	fakeRunner
	installed   map[string]string // agent bin -> resolved path
	outputs     map[string]string // tmux subcommand -> stdout
	interactive [][]string
}

func (f *fakeSessionRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	_, _ = f.fakeRunner.Output(ctx, name, args...)
	for sub, out := range f.outputs {
		if slices.Contains(args, sub) {
			return []byte(out), nil
		}
	}
	return nil, f.err
}

func (f *fakeSessionRunner) LookPath(name string) (string, error) {
	if path, ok := f.installed[name]; ok {
		return path, nil
	}
	return "", errors.New(name + " not found")
}

func (f *fakeSessionRunner) RunInteractive(_ context.Context, name string, args ...string) error {
	f.interactive = append(f.interactive, append([]string{name}, args...))
	return nil
}

// TestSessionName pins the repository-path to session-name derivation.
func TestSessionName(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{name: "plain basename", dir: "/Users/x/Repos/org/dotty", want: "dotty"},
		{name: "dots replaced", dir: "/Users/x/Repos/org/dotty.io", want: "dotty_io"},
		{name: "relative path", dir: "dotty", want: "dotty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SessionName(tt.dir); got != tt.want {
				t.Errorf("SessionName(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

// TestHasSession maps the tmux has-session exit status to a boolean.
func TestHasSession(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "session exists", err: nil, want: true},
		{name: "no session", err: errors.New("can't find session"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRunner{err: tt.err}
			if got := HasSession(context.Background(), f, "dotty"); got != tt.want {
				t.Errorf("HasSession = %v, want %v", got, tt.want)
			}
			want := []string{"tmux", "has-session", "-t", "dotty"}
			if len(f.calls) != 1 || !slices.Equal(f.calls[0], want) {
				t.Errorf("calls = %v, want [%v]", f.calls, want)
			}
		})
	}
}

// TestNewSession pins the full layout sequence: the editor session with its
// shell split, a window per installed agent only, and the trailing shell
// window.
func TestNewSession(t *testing.T) {
	f := &fakeSessionRunner{
		installed: map[string]string{
			"grok":   "/opt/homebrew/bin/grok",
			"codex":  "/opt/homebrew/bin/codex",
			"claude": "/opt/homebrew/bin/claude",
		},
		outputs: map[string]string{
			"new-session":     "%1\n",
			"display-message": "@1\n",
		},
	}
	if err := NewSession(context.Background(), f, "dotty", "/repo", "nvim"); err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	want := [][]string{
		{"tmux", "-u", "new-session", "-d", "-P", "-F", "#{pane_id}",
			"-s", "dotty", "-n", "  nvim", "-c", "/repo", "-x", "-", "-y", "-", "nvim", "."},
		{"tmux", "display-message", "-p", "-t", "%1", "#{window_id}"},
		{"tmux", "split-window", "-t", "%1", "-v", "-l", "10%", "-c", "/repo"},
		{"tmux", "select-pane", "-t", "%1"},
		{"tmux", "new-window", "-a", "-d", "-t", "@1", "-c", "/repo", "-n", "󴁹  grok", "/opt/homebrew/bin/grok"},
		{"tmux", "new-window", "-a", "-d", "-t", "@1", "-c", "/repo", "-n", "󴀾  codex", "/opt/homebrew/bin/codex"},
		{"tmux", "new-window", "-a", "-d", "-t", "@1", "-c", "/repo", "-n", "󴀶  claude", "/opt/homebrew/bin/claude"},
		{"tmux", "new-window", "-a", "-d", "-t", "@1", "-n", "  zsh", "-c", "/repo"},
	}
	if len(f.calls) != len(want) {
		t.Fatalf("calls = %d, want %d:\n%v", len(f.calls), len(want), f.calls)
	}
	for i := range want {
		if !slices.Equal(f.calls[i], want[i]) {
			t.Errorf("call %d = %v, want %v", i, f.calls[i], want[i])
		}
	}
}

// TestNewSessionEditorArgs threads a multi-word editor ($EDITOR="code --wait")
// into the first window, named after the editor's basename.
func TestNewSessionEditorArgs(t *testing.T) {
	f := &fakeSessionRunner{outputs: map[string]string{"new-session": "%1", "display-message": "@1"}}
	if err := NewSession(context.Background(), f, "s", "/repo", "/usr/local/bin/code", "--wait"); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	want := []string{"tmux", "-u", "new-session", "-d", "-P", "-F", "#{pane_id}",
		"-s", "s", "-n", "  code", "-c", "/repo", "-x", "-", "-y", "-", "/usr/local/bin/code", "--wait", "."}
	if !slices.Equal(f.calls[0], want) {
		t.Errorf("new-session call = %v, want %v", f.calls[0], want)
	}
}

// TestNewSessionFails surfaces a failed new-session as an error instead of
// continuing the layout against a session that does not exist.
func TestNewSessionFails(t *testing.T) {
	f := &fakeSessionRunner{fakeRunner: fakeRunner{err: errors.New("server exited")}}
	err := NewSession(context.Background(), f, "dotty", "/repo", "nvim")
	if err == nil || !strings.Contains(err.Error(), "create session dotty") {
		t.Errorf("NewSession error = %v, want create session failure", err)
	}
	if len(f.calls) != 1 {
		t.Errorf("calls = %v, want the layout abandoned after new-session", f.calls)
	}
}

// TestAttach pins the two attach modes: switch-client inside tmux (attach
// would nest), attach-session outside.
func TestAttach(t *testing.T) {
	tests := []struct {
		name       string
		insideTmux bool
		want       []string
	}{
		{
			name:       "inside tmux switches client",
			insideTmux: true,
			want:       []string{"tmux", "switch-client", "-t", "dotty"},
		},
		{
			name:       "outside tmux attaches",
			insideTmux: false,
			want:       []string{"tmux", "-u", "attach-session", "-t", "dotty", "-c", "/repo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeSessionRunner{}
			if err := Attach(context.Background(), f, "dotty", "/repo", tt.insideTmux); err != nil {
				t.Fatalf("Attach: %v", err)
			}
			if len(f.interactive) != 1 || !slices.Equal(f.interactive[0], tt.want) {
				t.Errorf("interactive calls = %v, want [%v]", f.interactive, tt.want)
			}
		})
	}
}

// TestFindRepos builds a fixture tree and checks matches are collected
// without descending into them, worktree-style .git files count, and the
// depth limit prunes the walk.
func TestFindRepos(t *testing.T) {
	root := t.TempDir()
	mk := func(parts ...string) string {
		t.Helper()
		p := filepath.Join(append([]string{root}, parts...)...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	mk("org", "repo-a", ".git")
	mk("org", "repo-a", "vendored", ".git") // inside a repo: not descended into
	mk("org", "not-a-repo")
	mk("deep", "a", "b", "c", "repo-d", ".git") // beyond maxDepth 4: pruned
	wt := mk("org", "worktree-b")
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := FindRepos(root, 4)
	want := []string{filepath.Join(root, "org", "repo-a"), filepath.Join(root, "org", "worktree-b")}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("FindRepos = %v, want %v", got, want)
	}
}

// BenchmarkFindRepos measures the repo walk over a tree shaped like a real
// ~/Repos: 20 orgs of 25 repos (each with content dirs that must not be
// descended into) plus deep non-repo noise that the depth limit prunes.
func BenchmarkFindRepos(b *testing.B) {
	root := b.TempDir()
	for org := range 20 {
		for repo := range 25 {
			dir := filepath.Join(root, fmt.Sprintf("org-%02d", org), fmt.Sprintf("repo-%02d", repo))
			for _, sub := range []string{".git", "src", "docs"} {
				if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "noise", "a", "b", "c", "d"), 0o755); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		if repos := FindRepos(root, 4); len(repos) != 500 {
			b.Fatalf("found %d repos, want 500", len(repos))
		}
	}
}
