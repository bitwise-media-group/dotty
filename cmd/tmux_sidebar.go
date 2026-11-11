// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwise-media-group/dotty/internal/cli"
	"github.com/bitwise-media-group/dotty/internal/tmux"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

// sidebarMemoryLimit is the sidebar's soft heap limit: it lives as long as
// the tmux server, so it trades a little GC for a small, steady footprint.
const sidebarMemoryLimit = 32 << 20

var tmuxSidebarCmd = &cobra.Command{
	Use:   "sidebar",
	Short: "Run the tmux session sidebar.",
	Long: `Show every tmux session and its windows as a tree in a narrow pane, with each
coding agent's status: 󰂚 needs attention, ● is waiting for input. A session
row's badge counts its windows in the most urgent state.

This is the process the sidebar pane runs; tmux starts it (through the
sidebar.conf hooks or the toggle), and there is one for the whole server.
It moves itself into whichever window the client switches to, blocks on a
tmux wait-for channel between changes, and reloads the tree only when a hook
or an agent status change signals it.

Keys: j/k move, h/l collapse and expand, n/N jump to the next or previous
window needing you (attention first, then waiting), / filters, enter jumps to
the session or window, esc returns focus, q hides the sidebar. Clicking a row
jumps to it.`,
	Example: `  dotty tmux sidebar toggle
  tmux split-window -h -f -l 32 'exec dotty tmux sidebar'`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		pane := os.Getenv("TMUX_PANE")
		if os.Getenv("TMUX") == "" || pane == "" {
			return errors.New("the sidebar runs inside a tmux pane; use dotty tmux sidebar toggle")
		}
		runtime.GOMAXPROCS(1)
		debug.SetMemoryLimit(sidebarMemoryLimit)

		ctx := cmd.Context()
		ios := cli.System()
		runner := newRunner(ios)
		won, err := runner.Output(ctx, "tmux", tmux.RegisterArgs(pane)...)
		if err != nil {
			return fmt.Errorf("register sidebar: %w", err)
		}
		if strings.TrimSpace(string(won)) != "1" {
			return nil // another sidebar started first; this pane closes
		}
		return tui.RunStatusTree(ctx, ios, "Sessions", sidebarOptions(runner, pane))
	},
}

var tmuxSidebarToggleFlags struct {
	pane string
}

var tmuxSidebarToggleCmd = &cobra.Command{
	Use:   "toggle",
	Short: "Show, focus, or hide the tmux session sidebar.",
	Long: `Cycle the session sidebar from the focused pane: hidden, it is shown and
focused; shown but unfocused, it is focused (moved into this window first if
it is elsewhere); focused, it is hidden. Hiding sets the @dotty_sidebar option
off, so switching windows no longer recreates it until the next toggle.

The tmux prefix binding (prefix s) and Neovim's session-sidebar mapping run
this.`,
	Example: `  dotty tmux sidebar toggle
  dotty tmux sidebar toggle --pane %3`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		runner := newRunner(cli.System())
		pane := tmuxSidebarToggleFlags.pane
		if pane == "" {
			pane = os.Getenv("TMUX_PANE")
		}
		st, err := tmux.ReadSidebarState(ctx, runner, pane)
		if err != nil {
			return err
		}
		if _, err := runner.Output(ctx, "tmux", tmux.ToggleArgs(st)...); err != nil {
			return fmt.Errorf("toggle sidebar: %w", err)
		}
		return nil
	},
}

// tmuxSidebarEnsureCmd is what the sidebar.conf follow hooks run when the
// sidebar is switched on but missing, never a user, hence hidden.
var tmuxSidebarEnsureCmd = &cobra.Command{
	Use:    "ensure",
	Short:  "Create the tmux session sidebar if it is on but missing (internal).",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		runner := newRunner(cli.System())
		window := tmuxOutput(ctx, runner, tmux.TargetArgs())
		if window == "" {
			window = tmuxOutput(ctx, runner, tmux.CurrentWindowArgs())
		}
		if window == "" {
			return errors.New("no tmux window to place the sidebar in")
		}
		if _, err := runner.Output(ctx, "tmux", tmux.EnsureArgs(window)...); err != nil {
			return fmt.Errorf("create sidebar: %w", err)
		}
		return nil
	},
}

func init() {
	tmuxSidebarToggleCmd.Flags().StringVar(&tmuxSidebarToggleFlags.pane, "pane", "",
		"the focused pane (defaults to $TMUX_PANE, else the current client's pane)")
	tmuxSidebarCmd.AddCommand(tmuxSidebarToggleCmd, tmuxSidebarEnsureCmd)
	tmuxCmd.AddCommand(tmuxSidebarCmd)
}

// sidebarOptions wires the status tree to tmux for the sidebar in pane.
func sidebarOptions(r tmux.Runner, pane string) tui.StatusTreeOptions {
	run := func(ctx context.Context, argv []string) {
		_, _ = r.Output(ctx, "tmux", argv...) // best effort: the next wake retries
	}
	return tui.StatusTreeOptions{
		Load: func(ctx context.Context) ([]tui.StatusNode, error) {
			sessions, err := tmux.ListTree(ctx, r)
			if err != nil {
				return nil, err
			}
			return sidebarNodes(sessions), nil
		},
		Wait: func(ctx context.Context) error {
			_, err := r.Output(ctx, "tmux", tmux.WaitArgs()...)
			return err
		},
		OnWake: func(ctx context.Context) {
			out, err := r.Output(ctx, "tmux", tmux.PlacementArgs(pane)...)
			if err != nil {
				return
			}
			p, err := tmux.ParsePlacement(out)
			if err != nil {
				return
			}
			if argv := tmux.PlaceArgs(pane, p); argv != nil {
				run(ctx, argv)
			}
		},
		OnSelect: func(ctx context.Context, value string) {
			session, window, _ := strings.Cut(value, ":")
			own := tmuxOutput(ctx, r, tmux.WindowOfArgs(pane))
			client := tmux.ClientViewing([]byte(tmuxOutput(ctx, r, tmux.ClientArgs())), own)
			run(ctx, tmux.SwitchArgs(client, session, window, pane))
		},
		OnBlur: func(ctx context.Context) { run(ctx, tmux.BlurArgs(pane)) },
		OnHide: func(ctx context.Context) { run(ctx, tmux.HideArgs()) },
	}
}

// sidebarNodes maps the tmux tree onto status-tree nodes. A session's Value
// is its id and a window's is "session:window", both tmux targets.
func sidebarNodes(sessions []tmux.Session) []tui.StatusNode {
	nodes := make([]tui.StatusNode, len(sessions))
	for i, s := range sessions {
		status, count := s.Status()
		node := tui.StatusNode{
			Label:   s.Name,
			Value:   s.ID,
			Status:  sidebarStatus(status),
			Count:   count,
			Current: s.Current(),
			Leaves:  make([]tui.StatusLeaf, len(s.Windows)),
		}
		for j, w := range s.Windows {
			node.Leaves[j] = tui.StatusLeaf{
				Label:   w.Index + ": " + strings.TrimSpace(w.Name),
				Value:   s.ID + ":" + w.ID,
				Status:  sidebarStatus(w.Status),
				Current: w.Current,
			}
		}
		nodes[i] = node
	}
	return nodes
}

func sidebarStatus(status string) tui.Status {
	switch status {
	case tmux.StatusAttention:
		return tui.StatusAttention
	case tmux.StatusWaiting:
		return tui.StatusWaiting
	}
	return tui.StatusNone
}

// tmuxOutput runs a tmux query, returning its trimmed output or "" on error.
func tmuxOutput(ctx context.Context, r tmux.Runner, argv []string) string {
	out, err := r.Output(ctx, "tmux", argv...)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
