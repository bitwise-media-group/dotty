// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tmux

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
)

// Agent statuses as set-status records them in the @agent_status window
// option; any other value (including none) means the agent needs nothing.
const (
	StatusWaiting   = "waiting"
	StatusAttention = "attention"
)

// sep separates TreeFormat fields: window names carry spaces and glyphs, so
// only a control character is safe.
const sep = "\x1f"

// TreeFormat is the list-windows -a format ParseTree reads, one line per
// window. The last field marks the window the client last switched to (the
// sidebar's @dotty_sidebar_target, which window formats inherit from the
// global session options).
const TreeFormat = "#{session_id}" + sep + "#{session_name}" + sep + "#{session_path}" + sep +
	"#{window_id}" + sep + "#{window_index}" + sep + "#{window_name}" + sep +
	"#{window_active}" + sep + "#{@agent_status}" + sep +
	"#{==:#{window_id},#{" + optTarget + "}}"

// treeFields is the number of fields in a TreeFormat line.
const treeFields = 9

// Window is one tmux window as the session tree shows it.
type Window struct {
	ID     string // global window id (@N)
	Index  string
	Name   string
	Active bool   // the session's current window
	Status string // StatusWaiting, StatusAttention, or ""
	// Current marks the window the client last switched to.
	Current bool
}

// Session is one tmux session and its windows in index order.
type Session struct {
	ID      string // global session id ($N)
	Name    string
	Path    string
	Windows []Window
}

// Current reports whether the session holds the window the client last
// switched to.
func (s Session) Current() bool {
	return slices.ContainsFunc(s.Windows, func(w Window) bool { return w.Current })
}

// Status returns the most urgent agent status across the session's windows
// (attention over waiting) and how many windows carry it; "" and 0 when no
// agent needs anything.
func (s Session) Status() (string, int) {
	var attention, waiting int
	for _, w := range s.Windows {
		switch w.Status {
		case StatusAttention:
			attention++
		case StatusWaiting:
			waiting++
		}
	}
	switch {
	case attention > 0:
		return StatusAttention, attention
	case waiting > 0:
		return StatusWaiting, waiting
	}
	return "", 0
}

// ParseTree parses list-windows -a output in TreeFormat into sessions:
// the current session first, then the rest by name, so worktree sessions
// (dotty-feat-x) sit right after their repository's. Malformed lines are
// skipped.
func ParseTree(out []byte) []Session {
	var sessions []Session
	index := map[string]int{}
	for line := range bytes.Lines(out) {
		f := strings.Split(strings.TrimRight(string(line), "\r\n"), sep)
		if len(f) != treeFields || f[0] == "" || f[3] == "" {
			continue
		}
		i, ok := index[f[0]]
		if !ok {
			i = len(sessions)
			index[f[0]] = i
			sessions = append(sessions, Session{ID: f[0], Name: f[1], Path: f[2]})
		}
		status := f[7]
		if status != StatusWaiting && status != StatusAttention {
			status = ""
		}
		sessions[i].Windows = append(sessions[i].Windows, Window{
			ID:      f[3],
			Index:   f[4],
			Name:    f[5],
			Active:  f[6] == "1",
			Status:  status,
			Current: f[8] == "1",
		})
	}
	slices.SortStableFunc(sessions, func(a, b Session) int {
		if ac, bc := a.Current(), b.Current(); ac != bc {
			if ac {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return sessions
}

// ListTree lists every session and window on the server.
func ListTree(ctx context.Context, r Runner) ([]Session, error) {
	out, err := r.Output(ctx, "tmux", "list-windows", "-a", "-F", TreeFormat)
	if err != nil {
		return nil, fmt.Errorf("list windows: %w", err)
	}
	return ParseTree(out), nil
}
