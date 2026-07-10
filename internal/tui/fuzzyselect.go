// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// FuzzySelect presents an fzf-style picker: typing fuzzy-matches and re-ranks
// the options (best match first, matched characters highlighted), and long
// lists scroll inside a bounded window. Enter returns the Value under the
// cursor; esc aborts with ErrAborted.
func FuzzySelect(ios cli.IOStreams, title string, options []Option) (string, error) {
	if !ios.IsInteractive() {
		return "", ErrNotInteractive
	}
	m := newFuzzyModel(title, options)
	m.hasDarkBg = detectDark(ios)
	p := tea.NewProgram(m, tea.WithInput(ios.In), tea.WithOutput(ios.ErrOut))
	final, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("run fuzzy select UI: %w", err)
	}
	fm := final.(fuzzyModel)
	if fm.aborted {
		return "", ErrAborted
	}
	return fm.value(), nil
}

// fuzzyRow is one visible candidate: the option index and the byte offsets of
// the filter's matched characters within its label (fuzzy.MatchedIndexes).
type fuzzyRow struct {
	option  int
	matched []int
}

type fuzzyModel struct {
	title     string
	options   []Option
	labels    []string // extracted once for fuzzy.Find
	rows      []fuzzyRow
	cursor    int // index into rows
	offset    int // first row inside the scroll window
	filter    string
	accepted  bool
	aborted   bool
	hasDarkBg bool // terminal background, for the ink-accent cursor/highlights
}

func newFuzzyModel(title string, options []Option) fuzzyModel {
	m := fuzzyModel{
		title:     title,
		options:   options,
		labels:    make([]string, len(options)),
		hasDarkBg: true, // default to dotty's dark surface until detected
	}
	for i, o := range options {
		m.labels[i] = o.Label
	}
	m.rebuildRows()
	return m
}

// rebuildRows recomputes the candidate rows for the current filter: every
// option in the given order when empty, otherwise fuzzy matches best first.
// The cursor returns to the top so enter always accepts the best match.
func (m *fuzzyModel) rebuildRows() {
	m.rows = m.rows[:0]
	if m.filter == "" {
		for i := range m.options {
			m.rows = append(m.rows, fuzzyRow{option: i})
		}
	} else {
		for _, match := range fuzzy.Find(m.filter, m.labels) {
			m.rows = append(m.rows, fuzzyRow{option: match.Index, matched: match.MatchedIndexes})
		}
	}
	m.cursor, m.offset = 0, 0
}

func (m fuzzyModel) value() string {
	if m.cursor >= len(m.rows) {
		return ""
	}
	return m.options[m.rows[m.cursor].option].Value
}

// Init implements tea.Model.
func (m fuzzyModel) Init() tea.Cmd { return nil }

// Update implements tea.Model: every printable key extends the filter (fzf
// style — there is no separate filtering mode), arrows or ctrl+p/n move the
// cursor, enter accepts, esc or ctrl+c aborts.
func (m fuzzyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "esc", "ctrl+c":
		m.aborted = true
		return m, tea.Quit
	case "enter":
		if len(m.rows) == 0 {
			return m, nil // nothing to accept; keep editing the filter
		}
		m.accepted = true
		return m, tea.Quit
	case "up", "ctrl+p":
		m.moveCursor(-1)
	case "down", "ctrl+n":
		m.moveCursor(1)
	case "backspace":
		if m.filter != "" {
			_, size := utf8.DecodeLastRuneInString(m.filter)
			m.filter = m.filter[:len(m.filter)-size]
			m.rebuildRows()
		}
	case "ctrl+u":
		if m.filter != "" {
			m.filter = ""
			m.rebuildRows()
		}
	default:
		if key.Text != "" {
			m.filter += key.Text
			m.rebuildRows()
		}
	}
	return m, nil
}

// moveCursor moves by delta, clamped, and scrolls the window to keep the
// cursor visible.
func (m *fuzzyModel) moveCursor(delta int) {
	m.cursor = min(max(m.cursor+delta, 0), max(len(m.rows)-1, 0))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+maxPromptRows {
		m.offset = m.cursor - maxPromptRows + 1
	}
}

// View implements tea.Model.
func (m fuzzyModel) View() tea.View {
	if m.accepted || m.aborted {
		return tea.NewView("")
	}
	var b strings.Builder
	accent := accentStyle(m.hasDarkBg)
	fmt.Fprintf(&b, "\n  %s\n", treeTitleStyle.Render(m.title))
	fmt.Fprintf(&b, "  %s %s %s\n", accent.Render("❯"), m.filter,
		treeDimStyle.Render(fmt.Sprintf("%d/%d", len(m.rows), len(m.options))))

	if len(m.rows) == 0 {
		b.WriteString(treeDimStyle.Render("  no matches\n"))
	}
	for i := m.offset; i < len(m.rows) && i < m.offset+maxPromptRows; i++ {
		cursor := "  "
		if i == m.cursor {
			cursor = accent.Render("❯ ")
		}
		row := m.rows[i]
		fmt.Fprintf(&b, "  %s%s\n", cursor, highlightMatches(m.labels[row.option], row.matched, accent))
	}
	if hidden := len(m.rows) - m.offset - maxPromptRows; hidden > 0 {
		fmt.Fprintf(&b, "  %s\n", treeDimStyle.Render(fmt.Sprintf("… %d more (type to narrow)", hidden)))
	}
	b.WriteString(treeDimStyle.Render("\n  type to filter · enter accept · esc cancel\n"))
	return tea.NewView(b.String())
}

// highlightMatches renders label with the matched byte offsets in the accent
// style, grouping consecutive bytes into one styled run.
func highlightMatches(label string, matched []int, accent lipgloss.Style) string {
	if len(matched) == 0 {
		return label
	}
	set := make(map[int]bool, len(matched))
	for _, i := range matched {
		set[i] = true
	}
	var b, run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			b.WriteString(accent.Render(run.String()))
			run.Reset()
		}
	}
	for i, r := range label {
		if set[i] {
			run.WriteRune(r)
			continue
		}
		flush()
		b.WriteRune(r)
	}
	flush()
	return b.String()
}
