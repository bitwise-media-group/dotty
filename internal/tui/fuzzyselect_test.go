// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func benchOptions(n int) []Option {
	options := make([]Option, n)
	for i := range options {
		options[i] = Option{
			Label: fmt.Sprintf("org-%02d/repository-name-%03d", i%10, i),
			Value: fmt.Sprintf("/Users/x/Repos/org-%02d/repository-name-%03d", i%10, i),
		}
	}
	return options
}

func testFuzzyOptions() []Option {
	return []Option{
		{Label: "acme/webapp", Value: "/r/acme/webapp"},
		{Label: "bitwise-media-group/dotty", Value: "/r/bitwise-media-group/dotty"},
		{Label: "dmccaffery/dotfiles", Value: "/r/dmccaffery/dotfiles"},
		{Label: "oss/xdripswift", Value: "/r/oss/xdripswift"},
	}
}

func pressFuzzy(m fuzzyModel, keys ...string) fuzzyModel {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEsc}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		case "ctrl+u":
			msg = tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
		case "up", "down":
			msg = tea.KeyPressMsg{Code: map[string]rune{"up": tea.KeyUp, "down": tea.KeyDown}[k]}
		default:
			msg = tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
		}
		next, _ := m.Update(msg)
		m = next.(fuzzyModel)
	}
	return m
}

func typeFuzzy(m fuzzyModel, s string) fuzzyModel {
	for _, r := range s {
		m = pressFuzzy(m, string(r))
	}
	return m
}

// TestFuzzyModelSubsequenceMatch pins the fzf-style contract a substring
// filter cannot satisfy: a scattered subsequence across path separators still
// matches, and enter accepts the best match without any cursor movement.
func TestFuzzyModelSubsequenceMatch(t *testing.T) {
	m := typeFuzzy(newFuzzyModel("t", testFuzzyOptions()), "bmgdot")
	if len(m.rows) == 0 {
		t.Fatal("subsequence bmgdot matched nothing")
	}
	m = pressFuzzy(m, "enter")
	if !m.accepted || m.value() != "/r/bitwise-media-group/dotty" {
		t.Errorf("accepted = %v, value = %q, want the bmg dotty repo", m.accepted, m.value())
	}
}

// TestFuzzyModelRanking pins score ordering: for the query "dot", the
// contiguous match at a word boundary outranks the scattered one.
func TestFuzzyModelRanking(t *testing.T) {
	m := typeFuzzy(newFuzzyModel("t", testFuzzyOptions()), "dot")
	if len(m.rows) < 2 {
		t.Fatalf("rows = %d, want at least dotty and dotfiles", len(m.rows))
	}
	first := m.labels[m.rows[0].option]
	if !strings.Contains(first, "dot") {
		t.Errorf("best match %q does not contain the contiguous query", first)
	}
	for _, row := range m.rows {
		if m.labels[row.option] == "acme/webapp" {
			t.Error("webapp matched the query dot")
		}
	}
}

// TestFuzzyModelNoMatchThenBackspace pins filter editing: a dead-end query
// yields zero rows and enter is ignored, backspace restores the matches.
func TestFuzzyModelNoMatchThenBackspace(t *testing.T) {
	m := typeFuzzy(newFuzzyModel("t", testFuzzyOptions()), "dotz")
	if len(m.rows) != 0 {
		t.Fatalf("rows = %d, want 0 for dead-end query", len(m.rows))
	}
	m = pressFuzzy(m, "enter")
	if m.accepted {
		t.Error("enter with no matches accepted")
	}
	m = pressFuzzy(m, "backspace")
	if len(m.rows) == 0 {
		t.Error("backspace did not restore matches")
	}
	m = pressFuzzy(m, "ctrl+u")
	if m.filter != "" || len(m.rows) != len(testFuzzyOptions()) {
		t.Errorf("ctrl+u left filter %q with %d rows", m.filter, len(m.rows))
	}
}

// TestFuzzyModelCursorAndAbort pins navigation and the abort path.
func TestFuzzyModelCursorAndAbort(t *testing.T) {
	m := pressFuzzy(newFuzzyModel("t", testFuzzyOptions()), "down", "down", "up")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	m = pressFuzzy(m, "enter")
	if m.value() != "/r/bitwise-media-group/dotty" {
		t.Errorf("value = %q, want the second option", m.value())
	}

	m2 := pressFuzzy(newFuzzyModel("t", testFuzzyOptions()), "esc")
	if !m2.aborted {
		t.Error("esc did not abort")
	}
}

// TestFuzzyModelWindow pins the scroll window: long lists render at most
// maxPromptRows candidates plus an overflow hint, and moving the cursor past
// the window scrolls it.
func TestFuzzyModelWindow(t *testing.T) {
	options := benchOptions(150)
	m := newFuzzyModel("pick", options)
	view := m.View().Content
	if rendered := strings.Count(view, "repository-name-"); rendered != maxPromptRows {
		t.Errorf("rendered %d candidates, want %d", rendered, maxPromptRows)
	}
	if !strings.Contains(view, "more (type to narrow)") {
		t.Error("view missing the overflow hint")
	}

	for range maxPromptRows + 5 {
		m = pressFuzzy(m, "down")
	}
	if m.offset == 0 {
		t.Error("cursor past the window did not scroll it")
	}
	if !strings.Contains(m.View().Content, options[m.cursor].Label) {
		t.Error("cursor row not visible after scrolling")
	}
}

// TestFuzzyModelView pins the chrome: title, match counter, and help line.
func TestFuzzyModelView(t *testing.T) {
	m := typeFuzzy(newFuzzyModel("Start a session for which repository?", testFuzzyOptions()), "dot")
	view := m.View().Content
	for _, want := range []string{"Start a session for which repository?", "2/4", "enter accept"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

// BenchmarkFuzzySelectView measures rendering one frame of the picker over a
// long list (window capped at maxPromptRows, matched runes highlighted).
func BenchmarkFuzzySelectView(b *testing.B) {
	m := typeFuzzy(newFuzzyModel("pick", benchOptions(150)), "repo")
	b.ResetTimer()
	for b.Loop() {
		_ = m.View()
	}
}

// BenchmarkFuzzySelectFilter measures one keystroke's re-rank of a long list
// (the fuzzy.Find pass rebuildRows runs on every filter edit).
func BenchmarkFuzzySelectFilter(b *testing.B) {
	m := newFuzzyModel("pick", benchOptions(150))
	m.filter = "repo149"
	b.ResetTimer()
	for b.Loop() {
		m.rebuildRows()
	}
}
