// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tmux

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Channel is the tmux wait-for channel the sidebar blocks on. set-status and
// the sidebar.conf hooks signal it whenever the tree may have changed; a
// signal sent while nothing waits marks the channel woken, so the next wait
// returns at once and no change is lost.
const Channel = "dotty-tree"

// SidebarWidth is the sidebar column width in cells.
const SidebarWidth = 32

// SidebarCommand is the pane command that runs the sidebar. The pane's start
// command doubles as a marker (see sidebarPaneFormat), so a sidebar counts as
// present from the instant its pane is created, before it registers.
const SidebarCommand = "exec dotty tmux sidebar"

// The tmux options the sidebar shares with sidebar.conf.
const (
	optEnabled = "@dotty_sidebar"        // on | off, global
	optTarget  = "@dotty_sidebar_target" // window the client last switched to, global
	optMarker  = "@dotty_is_sidebar"     // set on the sidebar pane itself
)

// sidebarPaneFormat is non-empty for a pane that is the sidebar, or is
// starting as one. sidebar.conf repeats it in the follow hooks.
const sidebarPaneFormat = "#{||:#{" + optMarker + "},#{m:*dotty tmux sidebar*,#{pane_start_command}}}"

// sidebarExistsFormat expands to "1" for every sidebar pane on the server, so
// it is empty exactly when there is none.
const sidebarExistsFormat = "#{S:#{W:#{P:#{?" + sidebarPaneFormat + ",1,}}}}"

// WaitArgs is the argv that blocks until Channel is signalled.
func WaitArgs() []string {
	return []string{"wait-for", Channel}
}

// RegisterArgs claims pane as the sidebar, atomically: the marker is set only
// when no other pane already carries it, and the trailing display prints "1"
// exactly when this pane won. Two sidebars racing to start therefore settle
// on one, and the loser exits.
func RegisterArgs(pane string) []string {
	others := "#{S:#{W:#{P:#{?#{&&:#{" + optMarker + "},#{!=:#{pane_id}," + pane + "}},1,}}}}"
	claim := "set -p -t " + quote(pane) + " " + optMarker + " 1 ; select-pane -t " + quote(pane) + " -T sessions"
	return []string{
		"if-shell", "-F", "#{==:" + others + ",}", claim,
		";", "display-message", "-p", "-t", pane, "#{" + optMarker + "}",
	}
}

// FollowArgs moves the sidebar pane into window as a full-height column on
// its right edge, leaving focus where it was.
func FollowArgs(pane, window string) []string {
	return []string{"join-pane", "-d", "-h", "-f", "-l", strconv.Itoa(SidebarWidth), "-s", pane, "-t", window}
}

// SwitchArgs points client at session, or at window within it when window is
// non-empty. When the sidebar pane holds focus (it was clicked, or entered to
// pick a row), focus returns to the pane used before it, so the jump lands in
// the picked window rather than in the sidebar.
func SwitchArgs(client, session, window, sidebar string) []string {
	target := session
	if window != "" {
		target = session + ":" + window
	}
	argv := make([]string, 0, 12)
	argv = append(argv, "switch-client")
	if client != "" {
		argv = append(argv, "-c", client)
	}
	return append(argv, "-t", target,
		";", "if-shell", "-F", "-t", sidebar, "#{pane_active}", "last-pane -t "+quote(sidebar))
}

// BlurArgs returns focus from the sidebar to the pane used before it.
func BlurArgs(sidebar string) []string {
	return []string{"last-pane", "-t", sidebar}
}

// HideArgs switches the sidebar off; the sidebar process exits after it, which
// closes its pane.
func HideArgs() []string {
	return []string{"set-option", "-g", optEnabled, "off"}
}

// SidebarState is what the toggle acts on: the focused pane and the sidebar,
// if one exists.
type SidebarState struct {
	Focus       string // the focused pane
	FocusWindow string // the window holding Focus
	Pane        string // the sidebar pane; "" when there is none
	PaneWindow  string // the window holding Pane
}

// ToggleArgs cycles the sidebar: hidden → shown and focused, shown elsewhere →
// moved here and focused, shown but unfocused → focused, focused → hidden.
func ToggleArgs(st SidebarState) []string {
	switch {
	case st.Pane == "":
		return []string{
			"set-option", "-g", optEnabled, "on",
			";", "split-window", "-h", "-f", "-l", strconv.Itoa(SidebarWidth), "-t", st.FocusWindow, SidebarCommand,
		}
	case st.Pane == st.Focus:
		return []string{"set-option", "-g", optEnabled, "off", ";", "kill-pane", "-t", st.Pane}
	case st.PaneWindow != st.FocusWindow:
		return append(FollowArgs(st.Pane, st.FocusWindow), ";", "select-pane", "-t", st.Pane)
	default:
		return []string{"select-pane", "-t", st.Pane}
	}
}

// EnsureArgs creates the sidebar in window, without taking focus, when it is
// switched on and no sidebar exists. The check and the split run as one tmux
// command sequence, so concurrent ensures create one sidebar between them.
func EnsureArgs(window string) []string {
	cond := "#{&&:#{==:#{" + optEnabled + "},on},#{==:" + sidebarExistsFormat + ",}}"
	split := "split-window -d -h -f -l " + strconv.Itoa(SidebarWidth) +
		" -t " + quote(window) + " " + quote(SidebarCommand)
	return []string{"if-shell", "-F", cond, split}
}

// TargetArgs prints the window the client last switched to (empty until a
// sidebar.conf follow hook first runs).
func TargetArgs() []string {
	return []string{"show-options", "-gqv", optTarget}
}

// CurrentWindowArgs prints the current client's window.
func CurrentWindowArgs() []string {
	return []string{"display-message", "-p", "#{window_id}"}
}

// WindowOfArgs prints the window holding pane.
func WindowOfArgs(pane string) []string {
	return []string{"display-message", "-p", "-t", pane, "#{window_id}"}
}

// stateFormat reads SidebarState in one display-message: the focused pane and
// its window, then "pane:window " for every sidebar pane.
const stateFormat = "#{pane_id}" + sep + "#{window_id}" + sep +
	"#{S:#{W:#{P:#{?" + sidebarPaneFormat + ",#{pane_id}:#{window_id} ,}}}}"

// SidebarStateArgs reads the state for the pane focus (the current client's
// pane when focus is empty).
func SidebarStateArgs(focus string) []string {
	if focus == "" {
		return []string{"display-message", "-p", stateFormat}
	}
	return []string{"display-message", "-p", "-t", focus, stateFormat}
}

// ParseSidebarState parses SidebarStateArgs output.
func ParseSidebarState(out []byte) (SidebarState, error) {
	f := strings.Split(strings.TrimSpace(string(out)), sep)
	if len(f) != 3 || f[0] == "" {
		return SidebarState{}, fmt.Errorf("parse sidebar state %q", out)
	}
	st := SidebarState{Focus: f[0], FocusWindow: f[1]}
	if first, _, _ := strings.Cut(f[2], " "); first != "" {
		st.Pane, st.PaneWindow, _ = strings.Cut(first, ":")
	}
	return st, nil
}

// ReadSidebarState reads the state for the pane focus.
func ReadSidebarState(ctx context.Context, r Runner, focus string) (SidebarState, error) {
	out, err := r.Output(ctx, "tmux", SidebarStateArgs(focus)...)
	if err != nil {
		return SidebarState{}, fmt.Errorf("read sidebar state: %w", err)
	}
	return ParseSidebarState(out)
}

// Placement is where the sidebar pane is and where it should be.
type Placement struct {
	Window  string // the window holding the sidebar
	Panes   int    // panes in that window, the sidebar included
	Session string // the session holding that window
	Target  string // the window the client last switched to; "" if unknown
	Other   string // another window of Session to retreat to; "" if none
}

// placementFormat reads Placement for the sidebar pane: its window, pane
// count and session, the follow target, and the session's other windows
// as "@N:last " entries.
const placementFormat = "#{window_id}" + sep + "#{window_panes}" + sep + "#{session_id}" + sep +
	"#{" + optTarget + "}" + sep +
	"#{W:#{window_id}:#{window_last_flag} }"

// PlacementArgs reads the Placement of the sidebar pane.
func PlacementArgs(pane string) []string {
	return []string{"display-message", "-p", "-t", pane, placementFormat}
}

// ParsePlacement parses PlacementArgs output.
func ParsePlacement(out []byte) (Placement, error) {
	f := strings.Split(strings.TrimSpace(string(out)), sep)
	if len(f) != 5 || f[0] == "" {
		return Placement{}, fmt.Errorf("parse sidebar placement %q", out)
	}
	panes, err := strconv.Atoi(f[1])
	if err != nil {
		return Placement{}, fmt.Errorf("parse sidebar placement panes %q: %w", f[1], err)
	}
	p := Placement{Window: f[0], Panes: panes, Session: f[2], Target: f[3]}
	for entry := range strings.FieldsSeq(f[4]) {
		id, last, _ := strings.Cut(entry, ":")
		if id == p.Window {
			continue
		}
		if p.Other == "" || last == "1" {
			p.Other = id
		}
	}
	return p, nil
}

// PlaceArgs is the move that keeps the sidebar with the client, or nil when
// it is already in place:
//   - a different target window: follow it there;
//   - alone in its window (every other pane exited): retreat into another
//     window of the session, so tmux closes the emptied window as it would
//     have without the sidebar, or close the window when it was the last.
func PlaceArgs(pane string, p Placement) []string {
	switch {
	case p.Target != "" && p.Target != p.Window:
		return FollowArgs(pane, p.Target)
	case p.Panes > 1:
		return nil
	case p.Other != "":
		return FollowArgs(pane, p.Other)
	default:
		return []string{"kill-window", "-t", p.Window}
	}
}

// ClientArgs lists every client with the window it views.
func ClientArgs() []string {
	return []string{"list-clients", "-F", "#{client_name}" + sep + "#{window_id}"}
}

// ClientViewing returns the first client in ClientArgs output viewing window,
// or "" when none is.
func ClientViewing(out []byte, window string) string {
	for line := range strings.Lines(string(out)) {
		name, win, ok := strings.Cut(strings.TrimRight(line, "\r\n"), sep)
		if ok && win == window {
			return name
		}
	}
	return ""
}

// quote single-quotes s for a command string tmux parses itself (the bodies
// of if-shell): session ids start with $, which tmux would expand, and pane
// ids with %. Ids and SidebarCommand never contain a single quote.
func quote(s string) string {
	return "'" + s + "'"
}
