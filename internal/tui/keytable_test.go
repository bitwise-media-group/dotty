// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testRows() []TableRow {
	return []TableRow{
		{Cells: []string{"111", "work", "ed25519", "alice"}, Value: "/k/alice"},
		{Cells: []string{"222", "backup", "ed25519", "bob"}, Value: "/k/bob"},
		{Cells: []string{"333", "", "ecdsa", "carol"}, Value: "/k/carol"},
	}
}

func pressTable(m tableModel, keys ...string) tableModel {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEsc}
		case "up":
			msg = tea.KeyPressMsg{Code: tea.KeyUp}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		default:
			msg = tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
		}
		next, _ := m.Update(msg)
		m = next.(tableModel)
	}
	return m
}

func TestTableModelNavigationAndSelect(t *testing.T) {
	m := newTableModel("t", []string{"SERIAL", "ALIASES", "TYPE", "USERNAME"}, testRows())
	m = pressTable(m, "down", "enter")
	if !m.accepted {
		t.Fatal("enter did not accept")
	}
	if got := m.rows[m.visible[m.cursor]].Value; got != "/k/bob" {
		t.Errorf("selected = %q, want /k/bob", got)
	}
}

func TestTableModelFilter(t *testing.T) {
	m := newTableModel("t", []string{"SERIAL", "ALIASES", "TYPE", "USERNAME"}, testRows())
	m = pressTable(m, "c", "a", "r")
	if len(m.visible) != 1 {
		t.Fatalf("visible = %v, want 1 row", m.visible)
	}
	m = pressTable(m, "enter")
	if got := m.rows[m.visible[m.cursor]].Value; got != "/k/carol" {
		t.Errorf("selected = %q, want /k/carol", got)
	}

	t.Run("backspace widens the filter", func(t *testing.T) {
		m := pressTable(newTableModel("t", []string{"A"}, testRows()), "x", "y")
		if len(m.visible) != 0 {
			t.Fatalf("visible = %v, want none", m.visible)
		}
		m = pressTable(m, "backspace", "backspace")
		if len(m.visible) != 3 {
			t.Errorf("visible = %v, want all rows back", m.visible)
		}
	})

	t.Run("enter with no matches does not accept", func(t *testing.T) {
		m := pressTable(newTableModel("t", []string{"A"}, testRows()), "z", "z", "enter")
		if m.accepted {
			t.Error("accepted with no visible rows")
		}
	})
}

func TestTableModelEscape(t *testing.T) {
	m := pressTable(newTableModel("t", []string{"A"}, testRows()), "esc")
	if m.accepted {
		t.Error("esc accepted")
	}
}

func TestRenderTable(t *testing.T) {
	out := RenderTable([]string{"SERIAL", "ALIASES"}, testRows())
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want header + 3 rows", len(lines))
	}
	if !strings.HasPrefix(lines[0], "SERIAL") || !strings.Contains(lines[1], "111") {
		t.Errorf("table:\n%s", out)
	}
}
