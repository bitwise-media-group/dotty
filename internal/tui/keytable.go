// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// TableRow is one selectable table line. Cells align under the headers;
// Value is what the caller gets back on selection.
type TableRow struct {
	Cells []string
	Value string
}

// FilterTable presents a fuzzy-filterable, selectable table: typing filters
// across all columns, enter returns the highlighted row's Value, esc exits
// with ok == false (the caller prints nothing).
func FilterTable(
	ios cli.IOStreams, title string, headers []string, rows []TableRow,
) (value string, ok bool, err error) {
	if !ios.IsInteractive() {
		return "", false, ErrNotInteractive
	}
	m := newTableModel(title, headers, rows)
	m.hasDarkBg = detectDark(ios)
	p := tea.NewProgram(m, tea.WithInput(ios.In), tea.WithOutput(ios.ErrOut))
	final, err := p.Run()
	if err != nil {
		return "", false, fmt.Errorf("run table UI: %w", err)
	}
	fm := final.(tableModel)
	if !fm.accepted || len(fm.visible) == 0 {
		return "", false, nil
	}
	return fm.rows[fm.visible[fm.cursor]].Value, true, nil
}

// RenderTable renders the rows as a plain aligned table — the non-interactive
// fallback for list output.
func RenderTable(headers []string, rows []TableRow) string {
	widths := columnWidths(headers, rows)
	var b strings.Builder
	writeRow := func(cells []string) {
		for i, w := range widths {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			fmt.Fprintf(&b, "%-*s", w+2, cell)
		}
		b.WriteString("\n")
	}
	writeRow(headers)
	for _, row := range rows {
		writeRow(row.Cells)
	}
	return b.String()
}

type tableModel struct {
	title     string
	headers   []string
	rows      []TableRow
	haystack  []string // concatenated cells per row, for filtering
	visible   []int    // indexes into rows, post-filter
	cursor    int
	filter    string
	accepted  bool
	hasDarkBg bool // terminal background, for the ink-accent cursor
}

func newTableModel(title string, headers []string, rows []TableRow) tableModel {
	m := tableModel{title: title, headers: headers, rows: rows, hasDarkBg: true}
	m.haystack = make([]string, len(rows))
	for i, row := range rows {
		m.haystack[i] = strings.Join(row.Cells, " ")
	}
	m.refilter()
	return m
}

func (m *tableModel) refilter() {
	if m.filter == "" {
		m.visible = make([]int, len(m.rows))
		for i := range m.rows {
			m.visible[i] = i
		}
	} else {
		matches := fuzzy.Find(m.filter, m.haystack)
		m.visible = make([]int, len(matches))
		for i, match := range matches {
			m.visible[i] = match.Index
		}
	}
	if m.cursor >= len(m.visible) {
		m.cursor = max(0, len(m.visible)-1)
	}
}

// Init implements tea.Model.
func (m tableModel) Init() tea.Cmd { return nil }

// Update implements tea.Model: typing filters, arrows navigate, enter
// selects, esc cancels.
func (m tableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "ctrl+c":
		return m, tea.Quit
	case "enter":
		if len(m.visible) > 0 {
			m.accepted = true
		}
		return m, tea.Quit
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "ctrl+n":
		if m.cursor < len(m.visible)-1 {
			m.cursor++
		}
	case "backspace":
		if m.filter != "" {
			m.filter = m.filter[:len(m.filter)-1]
			m.refilter()
		}
	default:
		if key.Text != "" {
			m.filter += key.Text
			m.refilter()
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m tableModel) View() tea.View {
	if m.accepted {
		return tea.NewView("")
	}
	widths := columnWidths(m.headers, m.rows)
	accent := accentStyle(m.hasDarkBg)
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s\n", treeTitleStyle.Render(m.title))
	fmt.Fprintf(&b, "  filter: %s\n\n", m.filter)

	b.WriteString("    ")
	for i, h := range m.headers {
		fmt.Fprintf(&b, "%-*s", widths[i]+2, h)
	}
	b.WriteString("\n")
	for vi, ri := range m.visible {
		cursor := "  "
		if vi == m.cursor {
			cursor = accent.Render("❯ ")
		}
		fmt.Fprintf(&b, "  %s", cursor)
		for i, cell := range m.rows[ri].Cells {
			fmt.Fprintf(&b, "%-*s", widths[i]+2, cell)
		}
		b.WriteString("\n")
	}
	if len(m.visible) == 0 {
		b.WriteString(treeDimStyle.Render("    no matches\n"))
	}
	b.WriteString(treeDimStyle.Render("\n  type to filter · enter select · esc quit\n"))
	return tea.NewView(b.String())
}

func columnWidths(headers []string, rows []TableRow) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row.Cells {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	return widths
}
