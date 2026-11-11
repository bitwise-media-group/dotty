// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"reflect"
	"testing"

	"github.com/bitwise-media-group/dotty/internal/tmux"
	"github.com/bitwise-media-group/dotty/internal/tui"
)

func TestSidebarNodes(t *testing.T) {
	sessions := []tmux.Session{
		{ID: "$1", Name: "dotty", Windows: []tmux.Window{
			{ID: "@1", Index: "1", Name: "  nvim", Active: true},
			{ID: "@2", Index: "2", Name: "claude", Status: tmux.StatusWaiting, Current: true},
			{ID: "@3", Index: "3", Name: "codex", Status: tmux.StatusAttention},
		}},
		{ID: "$2", Name: "other", Windows: []tmux.Window{{ID: "@4", Index: "1", Name: "zsh"}}},
	}
	want := []tui.StatusNode{
		{Label: "dotty", Value: "$1", Status: tui.StatusAttention, Count: 1, Current: true, Leaves: []tui.StatusLeaf{
			{Label: "1: nvim", Value: "$1:@1"},
			{Label: "2: claude", Value: "$1:@2", Status: tui.StatusWaiting, Current: true},
			{Label: "3: codex", Value: "$1:@3", Status: tui.StatusAttention},
		}},
		{Label: "other", Value: "$2", Leaves: []tui.StatusLeaf{{Label: "1: zsh", Value: "$2:@4"}}},
	}
	if got := sidebarNodes(sessions); !reflect.DeepEqual(got, want) {
		t.Errorf("sidebarNodes() =\n%+v\nwant\n%+v", got, want)
	}
}

// TestTmuxSidebarNeedsPane checks the pane process refuses to start outside
// tmux rather than drawing a sidebar in an ordinary terminal.
func TestTmuxSidebarNeedsPane(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	if err := execDotty(t, "tmux", "sidebar"); err == nil {
		t.Error("dotty tmux sidebar outside tmux = nil, want an error")
	}
}
