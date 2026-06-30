// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testNodes() []TreeNode {
	return []TreeNode{
		{Label: "111", Leaves: []TreeLeaf{{Label: "work — main", Value: "work"}, {Label: "primary", Value: "primary"}}},
		{Label: "222", Leaves: []TreeLeaf{{Label: "backup", Value: "backup"}}},
	}
}

func press(m treeModel, keys ...string) treeModel {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEsc}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		default:
			if len(k) == 1 {
				msg = tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
			} else {
				code := map[string]rune{"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight}[k]
				msg = tea.KeyPressMsg{Code: code}
			}
		}
		next, _ := m.Update(msg)
		m = next.(treeModel)
	}
	return m
}

func TestTreeModelRows(t *testing.T) {
	m := newTreeModel("t", testNodes())
	// 2 nodes + 3 leaves visible when fully expanded.
	if len(m.rows) != 5 {
		t.Fatalf("rows = %d, want 5", len(m.rows))
	}
}

func TestTreeModelToggleLeaf(t *testing.T) {
	m := newTreeModel("t", testNodes())
	m = press(m, "down", "space") // cursor onto first leaf, toggle
	if !slices.Equal(m.selectedValues(), []string{"work"}) {
		t.Errorf("selected = %v, want [work]", m.selectedValues())
	}
	m = press(m, "space") // toggle back off
	if len(m.selectedValues()) != 0 {
		t.Errorf("selected = %v, want empty", m.selectedValues())
	}
}

func TestTreeModelToggleNode(t *testing.T) {
	m := newTreeModel("t", testNodes())
	m = press(m, "space") // on the node row: select all children
	if got := m.selectedValues(); !slices.Equal(got, []string{"work", "primary"}) {
		t.Errorf("selected = %v, want both leaves of node 111", got)
	}
	m = press(m, "space") // all selected → deselect all
	if len(m.selectedValues()) != 0 {
		t.Errorf("selected = %v, want empty", m.selectedValues())
	}
}

func TestTreeModelCollapse(t *testing.T) {
	m := newTreeModel("t", testNodes())
	m = press(m, "h")     // collapse node 111
	if len(m.rows) != 3 { // node 111 + node 222 + backup leaf
		t.Fatalf("rows after collapse = %d, want 3", len(m.rows))
	}
	m = press(m, "l") // expand again
	if len(m.rows) != 5 {
		t.Fatalf("rows after expand = %d, want 5", len(m.rows))
	}
	// h on a leaf jumps to its parent.
	m = press(m, "down", "h")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want parent node row 0", m.cursor)
	}
}

func TestTreeModelFilter(t *testing.T) {
	m := newTreeModel("t", testNodes())
	m = press(m, "/", "b", "a", "c")
	// Only node 222 with its matching leaf remains visible.
	if len(m.rows) != 2 {
		t.Fatalf("rows under filter = %d, want 2 (%v)", len(m.rows), m.rows)
	}
	m = press(m, "enter") // exit filter entry, keep filter
	m = press(m, "down", "space")
	if got := m.selectedValues(); !slices.Equal(got, []string{"backup"}) {
		t.Errorf("selected = %v, want [backup]", got)
	}
	// esc in filter mode clears it instead of aborting.
	m = press(m, "/", "esc")
	if m.aborted {
		t.Error("esc during filtering aborted the model")
	}
	if len(m.rows) != 5 {
		t.Errorf("rows after clearing filter = %d, want 5", len(m.rows))
	}
}

func TestTreeModelAcceptAndAbort(t *testing.T) {
	m := press(newTreeModel("t", testNodes()), "down", "space", "enter")
	if !m.accepted || m.aborted {
		t.Errorf("accepted = %v, aborted = %v", m.accepted, m.aborted)
	}

	m2 := press(newTreeModel("t", testNodes()), "esc")
	if !m2.aborted {
		t.Error("esc did not abort")
	}
}

func TestTreeModelView(t *testing.T) {
	view := newTreeModel("Remove which aliases?", testNodes()).View().Content
	for _, want := range []string{"Remove which aliases?", "111", "work — main", "[ ]"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}
