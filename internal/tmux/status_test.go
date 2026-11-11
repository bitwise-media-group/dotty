// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tmux

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakeRunner struct {
	err   error
	calls [][]string // name followed by args, one slice per call
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return nil, f.err
}

// TestStatusArgs pins the exact tmux invocation each state produces — the
// contract theme.conf's @agent_status rendering and the sidebar's wake-ups
// depend on. Each write is guarded so a repeat of the current state is a
// no-op that signals nothing.
func TestStatusArgs(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  []string
	}{
		{
			name:  "waiting sets",
			state: "waiting",
			want: []string{
				"if-shell", "-F", "-t", "%3", "#{!=:#{@agent_status},waiting}",
				"set-window-option -t '%3' @agent_status waiting ; wait-for -S dotty-tree",
			},
		},
		{
			name:  "attention sets",
			state: "attention",
			want: []string{
				"if-shell", "-F", "-t", "%3", "#{!=:#{@agent_status},attention}",
				"set-window-option -t '%3' @agent_status attention ; wait-for -S dotty-tree",
			},
		},
		{
			name:  "clear unsets",
			state: "clear",
			want: []string{
				"if-shell", "-F", "-t", "%3", "#{!=:#{@agent_status},}",
				"set-window-option -t '%3' -u @agent_status ; wait-for -S dotty-tree",
			},
		},
		{
			name:  "unknown state unsets",
			state: "bogus",
			want: []string{
				"if-shell", "-F", "-t", "%3", "#{!=:#{@agent_status},}",
				"set-window-option -t '%3' -u @agent_status ; wait-for -S dotty-tree",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusArgs("%3", tt.state); !slices.Equal(got, tt.want) {
				t.Errorf("StatusArgs(%%3, %q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestSetStatus checks the runner is invoked as tmux with the state argv, and
// that a runner failure is swallowed rather than surfaced.
func TestSetStatus(t *testing.T) {
	f := &fakeRunner{err: errors.New("no server running")}
	SetStatus(context.Background(), f, "%3", "waiting")

	want := append([]string{"tmux"}, StatusArgs("%3", "waiting")...)
	if len(f.calls) != 1 || !slices.Equal(f.calls[0], want) {
		t.Errorf("SetStatus calls = %v, want [%v]", f.calls, want)
	}
}

// TestTitle pins the OSC 0 escape sequence for each state.
func TestTitle(t *testing.T) {
	tests := []struct {
		name  string
		state string
		dir   string
		want  string
	}{
		{name: "waiting glyph", state: "waiting", dir: "/Users/x/proj", want: "\033]0;● proj\007"},
		{name: "attention glyph", state: "attention", dir: "/Users/x/proj", want: "\033]0;󰂚 proj\007"},
		{name: "clear bare", state: "clear", dir: "/Users/x/proj", want: "\033]0;proj\007"},
		{name: "unknown state bare", state: "bogus", dir: "/Users/x/proj", want: "\033]0;proj\007"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Title(tt.state, tt.dir); got != tt.want {
				t.Errorf("Title(%q, %q) = %q, want %q", tt.state, tt.dir, got, tt.want)
			}
		})
	}
}
