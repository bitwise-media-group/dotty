// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// fakeTree records every callback the model triggers.
type fakeTree struct {
	nodes    []StatusNode
	loadErr  error
	loads    int
	wakes    int
	chosen   []string
	blurs    int
	hides    int
	debounce int // debounced loads scheduled
}

func (f *fakeTree) model() statusTreeModel {
	m := newStatusTreeModel("Sessions", statusActions{
		load:   func() ([]StatusNode, error) { f.loads++; return f.nodes, f.loadErr },
		wait:   func() error { return errors.New("not in tests") },
		wake:   func() { f.wakes++ },
		choose: func(v string) { f.chosen = append(f.chosen, v) },
		blur:   func() { f.blurs++ },
		hide:   func() { f.hides++ },
	}, 0)
	m.after = func(_ time.Duration, msg tea.Msg) tea.Cmd {
		f.debounce++
		return func() tea.Msg { return msg }
	}
	return m
}

// statusNodes is two sessions: "dotty" (current, an attention window and a
// waiting one) and "other" (one waiting window).
func statusNodes() []StatusNode {
	return []StatusNode{
		{Label: "dotty", Value: "$1", Status: StatusAttention, Count: 1, Current: true, Leaves: []StatusLeaf{
			{Label: "1: nvim", Value: "$1:@1"},
			{Label: "2: claude", Value: "$1:@2", Status: StatusWaiting, Current: true},
			{Label: "3: codex", Value: "$1:@3", Status: StatusAttention},
		}},
		{Label: "other", Value: "$2", Status: StatusWaiting, Count: 1, Leaves: []StatusLeaf{
			{Label: "1: grok", Value: "$2:@4", Status: StatusWaiting},
		}},
	}
}

// loaded returns a model that has applied f's first load.
func loaded(t *testing.T, f *fakeTree) statusTreeModel {
	t.Helper()
	m, _ := step(f.model(), loadedMsg{nodes: f.nodes})
	return m
}

func step(m statusTreeModel, msg tea.Msg) (statusTreeModel, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(statusTreeModel), cmd
}

// run executes cmd and everything it batches or sequences, returning the
// messages produced. tea.Quit shows up as a tea.QuitMsg.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, run(c)...)
		}
		return out
	default:
		if seq, ok := sequenceCmds(msg); ok {
			var out []tea.Msg
			for _, c := range seq {
				out = append(out, run(c)...)
			}
			return out
		}
		return []tea.Msg{msg}
	}
}

// sequenceCmds unpacks tea.Sequence's unexported message type, which is a
// []tea.Cmd underneath.
func sequenceCmds(msg tea.Msg) ([]tea.Cmd, bool) {
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.Slice || v.Type().Elem() != reflect.TypeFor[tea.Cmd]() {
		return nil, false
	}
	cmds := make([]tea.Cmd, v.Len())
	for i := range cmds {
		cmds[i] = v.Index(i).Interface().(tea.Cmd)
	}
	return cmds, true
}

func keys(m statusTreeModel, ks ...string) (statusTreeModel, []tea.Msg) {
	msgs := make([]tea.Msg, 0, len(ks))
	for _, k := range ks {
		var msg tea.KeyPressMsg
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEsc}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "up":
			msg = tea.KeyPressMsg{Code: tea.KeyUp}
		default:
			msg = tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
		}
		var cmd tea.Cmd
		m, cmd = step(m, msg)
		msgs = append(msgs, run(cmd)...)
	}
	return m, msgs
}

// cursorLabel is the label of the row under the cursor.
func cursorLabel(m statusTreeModel) string {
	row := m.rows[m.cursor]
	if row.leaf < 0 {
		return m.nodes[row.node].Label
	}
	return m.nodes[row.node].Leaves[row.leaf].Label
}

func TestStatusTreeFirstLoadLandsOnCurrent(t *testing.T) {
	m := loaded(t, &fakeTree{nodes: statusNodes()})
	if len(m.rows) != 6 {
		t.Fatalf("rows = %d, want 6 (2 sessions + 4 windows)", len(m.rows))
	}
	if got := cursorLabel(m); got != "2: claude" {
		t.Errorf("cursor on %q, want the current window 2: claude", got)
	}
}

func TestStatusTreeNavigateAndCollapse(t *testing.T) {
	m := loaded(t, &fakeTree{nodes: statusNodes()})
	m, _ = keys(m, "h") // on a leaf: jump to its session
	if got := cursorLabel(m); got != "dotty" {
		t.Fatalf("h on a leaf → %q, want dotty", got)
	}
	m, _ = keys(m, "h") // on the session: collapse it
	if len(m.rows) != 3 {
		t.Fatalf("rows after collapse = %d, want 3", len(m.rows))
	}
	m, _ = keys(m, "j")
	if got := cursorLabel(m); got != "other" {
		t.Errorf("j past a collapsed session → %q, want other", got)
	}
	m, _ = keys(m, "k", "l")
	if len(m.rows) != 6 {
		t.Errorf("rows after expand = %d, want 6", len(m.rows))
	}
	m, _ = keys(m, "G")
	if got := cursorLabel(m); got != "1: grok" {
		t.Errorf("G → %q, want the last row", got)
	}
	m, _ = keys(m, "j")
	if got := cursorLabel(m); got != "1: grok" {
		t.Errorf("j at the bottom → %q, want to stay put", got)
	}
}

func TestStatusTreeFilter(t *testing.T) {
	m := loaded(t, &fakeTree{nodes: statusNodes()})
	m, _ = keys(m, "/", "g", "r", "o")
	if len(m.rows) != 2 || cursorLabel(m) != "other" && cursorLabel(m) != "1: grok" {
		t.Fatalf("filter gro rows = %d (cursor %q), want other + grok", len(m.rows), cursorLabel(m))
	}
	m, _ = keys(m, "backspace", "backspace", "backspace", "o", "t", "t", "y")
	if len(m.rows) != 4 {
		t.Errorf("filter otty rows = %d, want dotty with all 3 windows", len(m.rows))
	}
	m, _ = keys(m, "esc")
	if m.filtering || m.filter != "" || len(m.rows) != 6 {
		t.Errorf("esc left filtering=%v filter=%q rows=%d", m.filtering, m.filter, len(m.rows))
	}
}

func TestStatusTreeFilterEnterSelects(t *testing.T) {
	f := &fakeTree{nodes: statusNodes()}
	m := loaded(t, f)
	m, _ = keys(m, "/", "g", "r", "o", "k", "down", "enter")
	if !slices.Equal(f.chosen, []string{"$2:@4"}) {
		t.Errorf("chosen = %v, want [$2:@4]", f.chosen)
	}
	if m.filtering || m.filter != "" {
		t.Errorf("after enter filtering=%v filter=%q, want both cleared", m.filtering, m.filter)
	}
}

func TestStatusTreeJumpFlagged(t *testing.T) {
	m := loaded(t, &fakeTree{nodes: statusNodes()})
	m, _ = keys(m, "g") // start from an unflagged row
	// Attention first, then waiting in tree order, wrapping around.
	want := []string{"3: codex", "2: claude", "1: grok", "3: codex"}
	for _, w := range want {
		m, _ = keys(m, "n")
		if got := cursorLabel(m); got != w {
			t.Fatalf("n → %q, want %q", got, w)
		}
	}
	m, _ = keys(m, "N")
	if got := cursorLabel(m); got != "1: grok" {
		t.Errorf("N from codex → %q, want to wrap back to grok", got)
	}
}

func TestStatusTreeJumpFlaggedOpensCollapsed(t *testing.T) {
	m := loaded(t, &fakeTree{nodes: statusNodes()})
	m, _ = keys(m, "G", "h", "h") // collapse "other"
	if len(m.rows) != 5 {
		t.Fatalf("rows = %d, want 5 with other collapsed", len(m.rows))
	}
	m, _ = keys(m, "N") // from a non-flagged row: the last flagged, grok
	if got := cursorLabel(m); got != "1: grok" {
		t.Errorf("N → %q, want grok, revealed", got)
	}
}

func TestStatusTreeEnterSelectsAndKeepsRunning(t *testing.T) {
	f := &fakeTree{nodes: statusNodes()}
	m := loaded(t, f)
	m, msgs := keys(m, "enter", "g", "enter")
	if !slices.Equal(f.chosen, []string{"$1:@2", "$1"}) {
		t.Errorf("chosen = %v, want [$1:@2 $1]", f.chosen)
	}
	for _, msg := range msgs {
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Error("enter quit the tree; it must keep running")
		}
	}
	if !m.loaded {
		t.Error("model lost its tree")
	}
}

func TestStatusTreeClick(t *testing.T) {
	f := &fakeTree{nodes: statusNodes()}
	m := loaded(t, f)
	click := func(x, y int) {
		var cmd tea.Cmd
		m, cmd = step(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
		run(cmd)
	}
	click(10, 5) // header is 1 line, so y=5 is row 4: "other"
	click(10, 2) // row 1: "1: nvim"
	click(10, 0) // the title: nothing
	click(10, 9) // below the rows: nothing
	if !slices.Equal(f.chosen, []string{"$2", "$1:@1"}) {
		t.Errorf("chosen = %v, want [$2 $1:@1]", f.chosen)
	}
	click(1, 1) // dotty's marker: collapse, no jump
	if len(m.rows) != 3 || len(f.chosen) != 2 {
		t.Errorf("marker click: rows = %d, chosen = %v; want collapse only", len(m.rows), f.chosen)
	}
}

func TestStatusTreeEscBlursAndQHides(t *testing.T) {
	f := &fakeTree{nodes: statusNodes()}
	m := loaded(t, f)
	m, msgs := keys(m, "esc")
	if f.blurs != 1 || len(msgs) != 0 {
		t.Errorf("esc: blurs = %d, msgs = %v; want one blur and no quit", f.blurs, msgs)
	}
	_, msgs = keys(m, "q")
	if f.hides != 1 {
		t.Errorf("hides = %d, want 1", f.hides)
	}
	if len(msgs) != 1 {
		t.Fatalf("q msgs = %v, want a quit", msgs)
	}
	if _, ok := msgs[0].(tea.QuitMsg); !ok {
		t.Errorf("q msg = %T, want tea.QuitMsg", msgs[0])
	}
}

func TestStatusTreeReloadKeepsCursorAndCollapse(t *testing.T) {
	f := &fakeTree{nodes: statusNodes()}
	m := loaded(t, f)
	m, _ = keys(m, "G", "h", "h") // collapse "other"
	m, _ = keys(m, "g", "j", "j", "j")
	if got := cursorLabel(m); got != "3: codex" {
		t.Fatalf("cursor on %q, want 3: codex", got)
	}

	// A new window appears above the cursor and codex is no longer flagged.
	next := statusNodes()
	next[0].Leaves = slices.Insert(next[0].Leaves, 0, StatusLeaf{Label: "0: zsh", Value: "$1:@0"})
	next[0].Leaves[3].Status = StatusNone
	m, _ = step(m, loadedMsg{nodes: next})
	if got := cursorLabel(m); got != "3: codex" {
		t.Errorf("cursor after reload on %q, want 3: codex", got)
	}
	if !m.collapsed["$2"] || len(m.rows) != 6 {
		t.Errorf("collapse lost: collapsed=%v rows=%d", m.collapsed, len(m.rows))
	}

	// The cursor's window closes: the cursor falls back to its session.
	gone := statusNodes()
	gone[0].Leaves = gone[0].Leaves[:2]
	m, _ = step(m, loadedMsg{nodes: gone})
	if got := cursorLabel(m); got != "dotty" {
		t.Errorf("cursor after its window closed on %q, want dotty", got)
	}
}

func TestStatusTreeUnchangedLoadSkipsRebuild(t *testing.T) {
	f := &fakeTree{nodes: statusNodes()}
	m := loaded(t, f)
	before := m.rebuilds
	m, _ = step(m, loadedMsg{nodes: statusNodes()})
	if m.rebuilds != before {
		t.Errorf("rebuilds = %d after an identical load, want %d", m.rebuilds, before)
	}
	changed := statusNodes()
	changed[1].Leaves[0].Status = StatusAttention
	m, _ = step(m, loadedMsg{nodes: changed})
	if m.rebuilds != before+1 {
		t.Errorf("rebuilds = %d after a change, want %d", m.rebuilds, before+1)
	}
}

func TestStatusTreeLoadErrorKeepsTree(t *testing.T) {
	m := loaded(t, &fakeTree{nodes: statusNodes()})
	m, _ = step(m, loadedMsg{err: errors.New("no server running")})
	if len(m.rows) != 6 {
		t.Errorf("rows = %d after a failed load, want the old tree kept", len(m.rows))
	}
	if view := m.View().Content; !strings.Contains(view, "no server") {
		t.Errorf("footer does not show the load error:\n%s", view)
	}
	m, _ = step(m, loadedMsg{nodes: statusNodes()})
	if m.err != nil {
		t.Errorf("err = %v after a good load, want cleared", m.err)
	}
}

// TestStatusTreeDebounce sends a burst of wakes: each runs OnWake, but only
// the first schedules a Load, which runs once when the debounce fires.
func TestStatusTreeDebounce(t *testing.T) {
	f := &fakeTree{nodes: statusNodes()}
	m := loaded(t, f)

	var due []tea.Msg
	for range 3 {
		var cmd tea.Cmd
		m, cmd = step(m, wakeMsg{})
		for _, msg := range run(cmd) {
			if _, ok := msg.(loadDueMsg); ok {
				due = append(due, msg)
			}
		}
	}
	if f.wakes != 3 {
		t.Errorf("OnWake ran %d times, want 3", f.wakes)
	}
	if f.debounce != 1 || len(due) != 1 {
		t.Fatalf("scheduled %d debounced loads (%d due), want 1", f.debounce, len(due))
	}

	m, cmd := step(m, due[0])
	if msgs := run(cmd); f.loads != 1 || len(msgs) != 1 {
		t.Fatalf("loads = %d, msgs = %v; want one Load", f.loads, msgs)
	}

	// After the load fires, the next wake schedules a fresh one.
	_, cmd = step(m, wakeMsg{})
	run(cmd)
	if f.debounce != 2 {
		t.Errorf("scheduled %d debounced loads after the burst, want 2", f.debounce)
	}
}

func TestStatusTreeWaitFailureQuits(t *testing.T) {
	m := loaded(t, &fakeTree{nodes: statusNodes()})
	m, cmd := step(m, waitFailMsg{err: errors.New("server exited")})
	if m.waitErr == nil {
		t.Error("waitErr not recorded")
	}
	if msgs := run(cmd); len(msgs) != 1 {
		t.Errorf("wait failure msgs = %v, want a quit", msgs)
	} else if _, ok := msgs[0].(tea.QuitMsg); !ok {
		t.Errorf("wait failure msg = %T, want tea.QuitMsg", msgs[0])
	}
}

func TestStatusTreeScrollsAndFits(t *testing.T) {
	nodes := make([]StatusNode, 0, 20)
	for i := range 20 {
		nodes = append(nodes, StatusNode{
			Label: strings.Repeat("s", 40), Value: string(rune('a' + i)), Status: StatusWaiting, Count: 1,
		})
	}
	m := loaded(t, &fakeTree{nodes: nodes})
	m, _ = step(m, tea.WindowSizeMsg{Width: 20, Height: 6})
	m, _ = keys(m, "G")
	if m.offset != 20-4 {
		t.Errorf("offset = %d, want 16 so the last row shows", m.offset)
	}
	lines := strings.Split(m.View().Content, "\n")
	if len(lines) != 6 {
		t.Errorf("view lines = %d, want 6", len(lines))
	}
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > 20 {
			t.Errorf("line %q is %d cells, wider than 20", l, w)
		}
	}
}
