// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// Status is how urgently a tree entry wants attention.
type Status int

// Status values, least to most urgent.
const (
	StatusNone Status = iota
	StatusWaiting
	StatusAttention
)

// StatusLeaf is one selectable entry under a StatusNode (a window).
type StatusLeaf struct {
	Label   string
	Value   string // handed to OnSelect
	Status  Status
	Current bool // the entry in view, highlighted
}

// StatusNode is one collapsible, selectable group (a session). Status and
// Count summarise its leaves for the badge shown on the node row.
type StatusNode struct {
	Label   string
	Value   string // handed to OnSelect; also keys the collapse state
	Status  Status
	Count   int
	Current bool
	Leaves  []StatusLeaf
}

// StatusTreeOptions wires RunStatusTree to its data source. Every callback
// runs off the UI goroutine, so a slow tmux call never stalls input.
type StatusTreeOptions struct {
	// Load fetches the whole tree. An error shows in the footer and the
	// previous tree stays up.
	Load func(context.Context) ([]StatusNode, error)
	// Wait blocks until something may have changed. An error ends the tree.
	Wait func(context.Context) error
	// OnWake runs immediately on every wake, before the debounced Load.
	OnWake func(context.Context)
	// OnSelect runs for enter or a click on a row; the tree keeps running.
	OnSelect func(context.Context, string)
	// OnBlur runs for esc, to hand focus back.
	OnBlur func(context.Context)
	// OnHide runs for q before the tree exits.
	OnHide func(context.Context)
	// Debounce coalesces wakes: Load runs once this long after the first
	// wake of a burst. Zero means 150ms.
	Debounce time.Duration
}

// defaultDebounce is the StatusTreeOptions.Debounce default.
const defaultDebounce = 150 * time.Millisecond

// RunStatusTree shows a long-lived, single-select status tree that reloads
// whenever Wait returns. It returns nil when the user hides it with q, and an
// error when Wait fails (the data source went away).
func RunStatusTree(ctx context.Context, ios cli.IOStreams, title string, opts StatusTreeOptions) error {
	if !ios.IsInteractive() {
		return ErrNotInteractive
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // kills a Wait still blocked in its child process

	m := newStatusTreeModel(title, opts.actions(ctx), opts.Debounce)
	p := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(ios.In), tea.WithOutput(ios.ErrOut))
	final, err := p.Run()
	if err != nil {
		if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
			return nil // cancelled from outside: a normal shutdown
		}
		return fmt.Errorf("run status tree UI: %w", err)
	}
	if fm := final.(statusTreeModel); fm.waitErr != nil {
		return fmt.Errorf("wait for changes: %w", fm.waitErr)
	}
	return nil
}

// statusActions are the options bound to RunStatusTree's context, with nil
// callbacks replaced by no-ops, so the model never handles a context itself.
type statusActions struct {
	load       func() ([]StatusNode, error)
	wait       func() error
	wake       func()
	choose     func(string)
	blur, hide func()
}

func (o StatusTreeOptions) actions(ctx context.Context) statusActions {
	a := statusActions{
		load:   func() ([]StatusNode, error) { return nil, nil },
		wait:   func() error { <-ctx.Done(); return ctx.Err() },
		wake:   func() {},
		choose: func(string) {},
		blur:   func() {},
		hide:   func() {},
	}
	if o.Load != nil {
		a.load = func() ([]StatusNode, error) { return o.Load(ctx) }
	}
	if o.Wait != nil {
		a.wait = func() error { return o.Wait(ctx) }
	}
	if o.OnWake != nil {
		a.wake = func() { o.OnWake(ctx) }
	}
	if o.OnSelect != nil {
		a.choose = func(v string) { o.OnSelect(ctx, v) }
	}
	if o.OnBlur != nil {
		a.blur = func() { o.OnBlur(ctx) }
	}
	if o.OnHide != nil {
		a.hide = func() { o.OnHide(ctx) }
	}
	return a
}

type (
	wakeMsg     struct{}            // Wait returned
	loadDueMsg  struct{}            // the debounce after a wake elapsed
	waitFailMsg struct{ err error } // Wait failed; the tree ends
	loadedMsg   struct {            // Load returned
		nodes []StatusNode
		err   error
	}
)

type statusTreeModel struct {
	title    string
	act      statusActions
	debounce time.Duration
	// after schedules msg once d has elapsed; tests replace tea.Tick.
	after func(d time.Duration, msg tea.Msg) tea.Cmd

	nodes     []StatusNode
	loaded    bool // a Load has succeeded at least once
	rebuilds  int  // tree content changes applied, for tests
	collapsed map[string]bool
	rows      []treeRow
	cursor    int
	offset    int // first row on screen
	filter    string
	filtering bool

	width, height int
	hasDarkBg     bool
	loadPending   bool // a debounced Load is scheduled
	err           error
	waitErr       error
}

func newStatusTreeModel(title string, act statusActions, debounce time.Duration) statusTreeModel {
	if debounce <= 0 {
		debounce = defaultDebounce
	}
	return statusTreeModel{
		title:    title,
		act:      act,
		debounce: debounce,
		after: func(d time.Duration, msg tea.Msg) tea.Cmd {
			return tea.Tick(d, func(time.Time) tea.Msg { return msg })
		},
		collapsed: map[string]bool{},
		width:     32,
		height:    24,
		hasDarkBg: true, // dotty's dark surface until the terminal answers
	}
}

func (m statusTreeModel) waitCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.act.wait(); err != nil {
			return waitFailMsg{err: err}
		}
		return wakeMsg{}
	}
}

func (m statusTreeModel) wakeCmd() tea.Cmd {
	return func() tea.Msg { m.act.wake(); return nil }
}

func (m statusTreeModel) loadCmd() tea.Cmd {
	return func() tea.Msg {
		nodes, err := m.act.load()
		return loadedMsg{nodes: nodes, err: err}
	}
}

func (m statusTreeModel) chooseCmd(value string) tea.Cmd {
	return func() tea.Msg { m.act.choose(value); return nil }
}

// Init implements tea.Model: wake once (so the caller's follow step runs at
// start), load at once rather than after a debounce, and arm the waiter.
func (m statusTreeModel) Init() tea.Cmd {
	return tea.Batch(m.wakeCmd(), m.loadCmd(), m.waitCmd(), tea.RequestBackgroundColor)
}

// Update implements tea.Model.
func (m statusTreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.update(msg)
	m.scroll()
	return m, cmd
}

func (m *statusTreeModel) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.hasDarkBg = msg.IsDark()
	case wakeMsg:
		cmds := []tea.Cmd{m.wakeCmd(), m.waitCmd()}
		if !m.loadPending {
			m.loadPending = true
			cmds = append(cmds, m.after(m.debounce, loadDueMsg{}))
		}
		return tea.Batch(cmds...)
	case loadDueMsg:
		// Cleared before loading so a wake during the load schedules another:
		// the change behind it may postdate what this load reads.
		m.loadPending = false
		return m.loadCmd()
	case loadedMsg:
		m.applyLoad(msg.nodes, msg.err)
	case waitFailMsg:
		m.waitErr = msg.err
		return tea.Quit
	case tea.MouseClickMsg:
		return m.click(msg.Mouse())
	case tea.MouseWheelMsg:
		switch msg.Mouse().Button {
		case tea.MouseWheelUp:
			m.move(-1)
		case tea.MouseWheelDown:
			m.move(1)
		}
	case tea.KeyPressMsg:
		if m.filtering {
			return m.filterKey(msg)
		}
		return m.key(msg)
	}
	return nil
}

// applyLoad swaps in a freshly loaded tree, keeping the cursor on the same
// entry and the collapse state, and skipping the rebuild when nothing changed.
func (m *statusTreeModel) applyLoad(nodes []StatusNode, err error) {
	if err != nil {
		m.err = err
		return
	}
	m.err = nil
	if m.loaded && nodesEqual(m.nodes, nodes) {
		return
	}
	leaf, node := m.cursorValues()
	first := !m.loaded
	m.nodes, m.loaded = nodes, true
	m.rebuilds++
	m.rebuildRows()
	if first {
		m.cursorToCurrent()
		return
	}
	m.restoreCursor(leaf, node)
}

func nodesEqual(a, b []StatusNode) bool {
	return slices.EqualFunc(a, b, func(x, y StatusNode) bool {
		return x.Label == y.Label && x.Value == y.Value && x.Status == y.Status &&
			x.Count == y.Count && x.Current == y.Current && slices.Equal(x.Leaves, y.Leaves)
	})
}

func (m *statusTreeModel) key(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "q", "ctrl+c":
		return tea.Sequence(func() tea.Msg { m.act.hide(); return nil }, tea.Quit)
	case "esc":
		if m.filter != "" {
			m.setFilter("")
			return nil
		}
		return func() tea.Msg { m.act.blur(); return nil }
	case "enter":
		return m.chooseAtCursor()
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = max(0, len(m.rows)-1)
	case "left", "h":
		m.collapseAtCursor()
	case "right", "l":
		m.expandAtCursor()
	case "n":
		m.jumpFlagged(1)
	case "N":
		m.jumpFlagged(-1)
	case "/":
		m.filtering = true
	}
	return nil
}

// filterKey edits the filter. Enter picks the row under the cursor and drops
// the filter, so "/name⏎" jumps straight to a session.
func (m *statusTreeModel) filterKey(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		m.filtering = false
		m.setFilter("")
	case "enter":
		m.filtering = false
		cmd := m.chooseAtCursor()
		m.setFilter("")
		return cmd
	case "up", "ctrl+p":
		m.move(-1)
	case "down", "ctrl+n":
		m.move(1)
	case "backspace":
		if m.filter != "" {
			r := []rune(m.filter)
			m.setFilter(string(r[:len(r)-1]))
		}
	case "ctrl+c":
		return tea.Sequence(func() tea.Msg { m.act.hide(); return nil }, tea.Quit)
	default:
		if key.Text != "" {
			m.setFilter(m.filter + key.Text)
		}
	}
	return nil
}

// setFilter re-filters, keeping the cursor on the same entry when it stays
// visible.
func (m *statusTreeModel) setFilter(f string) {
	leaf, node := m.cursorValues()
	m.filter = f
	m.rebuildRows()
	m.restoreCursor(leaf, node)
}

// click picks the clicked row; a click on a node's ▾/▸ marker toggles it
// instead.
func (m *statusTreeModel) click(mouse tea.Mouse) tea.Cmd {
	if mouse.Button != tea.MouseLeft {
		return nil
	}
	i := m.offset + mouse.Y - m.headerHeight()
	if mouse.Y < m.headerHeight() || i < 0 || i >= len(m.rows) || i >= m.offset+m.listHeight() {
		return nil
	}
	m.cursor = i
	if row := m.rows[i]; row.leaf == -1 && mouse.X < 4 {
		if m.collapsed[m.nodes[row.node].Value] {
			m.expandAtCursor()
		} else {
			m.collapseAtCursor()
		}
		return nil
	}
	return m.chooseAtCursor()
}

func (m *statusTreeModel) chooseAtCursor() tea.Cmd {
	leaf, node := m.cursorValues()
	switch {
	case leaf != "":
		return m.chooseCmd(leaf)
	case node != "":
		return m.chooseCmd(node)
	}
	return nil
}

func (m *statusTreeModel) move(delta int) {
	m.cursor = min(max(m.cursor+delta, 0), max(0, len(m.rows)-1))
}

// rebuildRows recomputes the visible rows from the collapse state and filter.
// A filter keeps every leaf of a node whose label matches and otherwise the
// matching leaves, hides nodes with neither, and forces the rest open.
func (m *statusTreeModel) rebuildRows() {
	m.rows = m.rows[:0]
	for ni, node := range m.nodes {
		if m.filter == "" {
			m.rows = append(m.rows, treeRow{node: ni, leaf: -1})
			if !m.collapsed[node.Value] {
				for li := range node.Leaves {
					m.rows = append(m.rows, treeRow{node: ni, leaf: li})
				}
			}
			continue
		}
		var kept []int
		if len(fuzzy.Find(m.filter, []string{node.Label})) > 0 {
			for li := range node.Leaves {
				kept = append(kept, li)
			}
		} else {
			labels := make([]string, len(node.Leaves))
			for i, l := range node.Leaves {
				labels[i] = l.Label
			}
			matches := fuzzy.Find(m.filter, labels)
			if len(matches) == 0 {
				continue
			}
			for _, match := range matches {
				kept = append(kept, match.Index)
			}
			slices.Sort(kept) // keep window order, not match score order
		}
		m.rows = append(m.rows, treeRow{node: ni, leaf: -1})
		for _, li := range kept {
			m.rows = append(m.rows, treeRow{node: ni, leaf: li})
		}
	}
	m.cursor = min(m.cursor, max(0, len(m.rows)-1))
}

// cursorValues returns the leaf and node Values under the cursor (leaf is ""
// on a node row).
func (m statusTreeModel) cursorValues() (leaf, node string) {
	if m.cursor >= len(m.rows) {
		return "", ""
	}
	row := m.rows[m.cursor]
	n := m.nodes[row.node]
	if row.leaf >= 0 {
		return n.Leaves[row.leaf].Value, n.Value
	}
	return "", n.Value
}

// restoreCursor puts the cursor back on leaf, else on its node (the leaf
// went away), else leaves it clamped where it was.
func (m *statusTreeModel) restoreCursor(leaf, node string) {
	isLeaf := func(_ StatusNode, l *StatusLeaf) bool { return l != nil && l.Value == leaf }
	if i := m.rowIndex(isLeaf); leaf != "" && i >= 0 {
		m.cursor = i
		return
	}
	isNode := func(n StatusNode, l *StatusLeaf) bool { return l == nil && n.Value == node }
	if i := m.rowIndex(isNode); node != "" && i >= 0 {
		m.cursor = i
	}
}

// cursorToCurrent puts the cursor on the entry in view.
func (m *statusTreeModel) cursorToCurrent() {
	if i := m.rowIndex(func(_ StatusNode, l *StatusLeaf) bool { return l != nil && l.Current }); i >= 0 {
		m.cursor = i
		return
	}
	if i := m.rowIndex(func(n StatusNode, l *StatusLeaf) bool { return l == nil && n.Current }); i >= 0 {
		m.cursor = i
	}
}

// rowIndex returns the first visible row matching, or -1. leaf is nil for a
// node row.
func (m statusTreeModel) rowIndex(match func(node StatusNode, leaf *StatusLeaf) bool) int {
	return slices.IndexFunc(m.rows, func(r treeRow) bool {
		n := m.nodes[r.node]
		if r.leaf < 0 {
			return match(n, nil)
		}
		return match(n, &n.Leaves[r.leaf])
	})
}

func (m *statusTreeModel) collapseAtCursor() {
	if len(m.rows) == 0 || m.filter != "" {
		return
	}
	row := m.rows[m.cursor]
	if row.leaf >= 0 {
		m.cursor = slices.Index(m.rows, treeRow{node: row.node, leaf: -1})
		return
	}
	m.collapsed[m.nodes[row.node].Value] = true
	m.rebuildRows()
}

func (m *statusTreeModel) expandAtCursor() {
	if len(m.rows) == 0 || m.filter != "" {
		return
	}
	if row := m.rows[m.cursor]; row.leaf == -1 {
		delete(m.collapsed, m.nodes[row.node].Value)
		m.rebuildRows()
	}
}

// jumpFlagged moves the cursor to the next (dir 1) or previous (dir -1) leaf
// needing the user: every attention leaf in tree order, then every waiting
// one, wrapping around. A collapsed node holding the target opens.
func (m *statusTreeModel) jumpFlagged(dir int) {
	var flagged []treeRow
	for _, status := range []Status{StatusAttention, StatusWaiting} {
		for ni, node := range m.nodes {
			for li, leaf := range node.Leaves {
				r := treeRow{node: ni, leaf: li}
				if leaf.Status == status && (m.filter == "" || slices.Contains(m.rows, r)) {
					flagged = append(flagged, r)
				}
			}
		}
	}
	if len(flagged) == 0 {
		return
	}
	next := 0
	if dir < 0 {
		next = len(flagged) - 1
	}
	if m.cursor < len(m.rows) {
		if i := slices.Index(flagged, m.rows[m.cursor]); i >= 0 {
			next = (i + dir + len(flagged)) % len(flagged)
		}
	}
	target := flagged[next]
	if m.collapsed[m.nodes[target.node].Value] {
		delete(m.collapsed, m.nodes[target.node].Value)
		m.rebuildRows()
	}
	m.cursor = slices.Index(m.rows, target)
}

// headerHeight is the title line plus the filter line while one shows.
func (m statusTreeModel) headerHeight() int {
	if m.filtering || m.filter != "" {
		return 2
	}
	return 1
}

// listHeight is the rows that fit between the header and the footer line.
func (m statusTreeModel) listHeight() int {
	return max(1, m.height-m.headerHeight()-1)
}

// scroll keeps the cursor on screen.
func (m *statusTreeModel) scroll() {
	h := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = min(max(m.offset, 0), max(0, len(m.rows)-h))
}

var (
	statusTitleStyle   = lipgloss.NewStyle().Bold(true)
	statusDimStyle     = lipgloss.NewStyle().Faint(true)
	statusWaitStyle    = lipgloss.NewStyle().Foreground(colorWarning)
	statusAlertStyle   = lipgloss.NewStyle().Foreground(colorDanger).Bold(true)
	statusErrorStyle   = lipgloss.NewStyle().Foreground(colorDanger)
	statusNodeStyle    = lipgloss.NewStyle().Bold(true)
	statusWaitingGlyph = "●"
	statusAlertGlyph   = "󰂚"
)

// View implements tea.View: a title, the visible rows, and a one-line footer
// (the key hint, or the last Load error), each line fitted to the width.
func (m statusTreeModel) View() tea.View {
	w := max(m.width, 8)
	accent := accentStyle(m.hasDarkBg)
	current := accent.Bold(true)
	var lines []string

	lines = append(lines, fit(" "+statusTitleStyle.Render(m.title), w))
	if m.filtering || m.filter != "" {
		caret := ""
		if m.filtering {
			caret = accent.Render("▏")
		}
		lines = append(lines, fit(" / "+m.filter+caret, w))
	}

	end := min(m.offset+m.listHeight(), len(m.rows))
	for i := m.offset; i < end; i++ {
		row := m.rows[i]
		cursor := "  "
		if i == m.cursor {
			cursor = accent.Render("❯ ")
		}
		node := m.nodes[row.node]
		if row.leaf == -1 {
			marker := "▾ "
			if m.collapsed[node.Value] && m.filter == "" {
				marker = "▸ "
			}
			style := statusNodeStyle
			if node.Current {
				style = current
			}
			badge, badgeStyle := "", lipgloss.NewStyle()
			if node.Count > 0 {
				badge, badgeStyle = statusBadge(node.Status, fmt.Sprintf(" %d", node.Count))
			}
			lines = append(lines, row2(cursor+marker, node.Label, style, badge, badgeStyle, w))
			continue
		}
		leaf := node.Leaves[row.leaf]
		style := lipgloss.NewStyle()
		if leaf.Current {
			style = current
		}
		glyph, glyphStyle := statusBadge(leaf.Status, "")
		lines = append(lines, row2(cursor+"  ", leaf.Label, style, glyph, glyphStyle, w))
	}
	for len(lines) < m.headerHeight()+m.listHeight() {
		lines = append(lines, "")
	}

	if m.err != nil {
		lines = append(lines, fit(" "+statusErrorStyle.Render(ansi.Truncate(m.err.Error(), w-1, "…")), w))
	} else {
		lines = append(lines, statusDimStyle.Render(ansi.Truncate(" ⏎ jump  n next  / find  q hide", w, "…")))
	}

	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// statusBadge is the glyph (plus suffix) and style for status; "" for none.
func statusBadge(status Status, suffix string) (string, lipgloss.Style) {
	switch status {
	case StatusAttention:
		return statusAlertGlyph + suffix, statusAlertStyle
	case StatusWaiting:
		return statusWaitingGlyph + suffix, statusWaitStyle
	}
	return "", lipgloss.NewStyle()
}

// row2 lays out prefix and a label truncated to leave room for right, which
// sits right-aligned one cell from the edge.
func row2(prefix, label string, labelStyle lipgloss.Style, right string, rightStyle lipgloss.Style, width int) string {
	rw := ansi.StringWidth(right)
	room := width - ansi.StringWidth(prefix) - 1
	if rw > 0 {
		room -= rw + 1
	}
	label = ansi.Truncate(label, max(room, 1), "…")
	pad := max(width-1-ansi.StringWidth(prefix)-ansi.StringWidth(label)-rw, 0)
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(labelStyle.Render(label))
	b.WriteString(strings.Repeat(" ", pad))
	if rw > 0 {
		b.WriteString(rightStyle.Render(right))
	}
	return b.String()
}

// fit truncates an already-styled line to width.
func fit(s string, width int) string {
	return ansi.Truncate(s, width, "")
}
