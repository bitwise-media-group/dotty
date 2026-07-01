// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// TreeLeaf is one selectable entry under a tree node.
type TreeLeaf struct {
	Label string // what the user sees
	Value string // what the caller gets back
}

// TreeNode is one collapsible group (a security-key serial) with selectable
// leaves (its aliases).
type TreeNode struct {
	Label  string
	Leaves []TreeLeaf
}

// TreeMultiSelect presents a collapsible, fuzzy-filterable tree and returns
// the Values of the leaves selected when the user accepts with enter. Esc
// returns ErrAborted.
func TreeMultiSelect(ios cli.IOStreams, title string, nodes []TreeNode) ([]string, error) {
	if !ios.IsInteractive() {
		return nil, ErrNotInteractive
	}
	m := newTreeModel(title, nodes)
	m.hasDarkBg = detectDark(ios)
	p := tea.NewProgram(m, tea.WithInput(ios.In), tea.WithOutput(ios.ErrOut))
	final, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("run tree select UI: %w", err)
	}
	fm := final.(treeModel)
	if fm.aborted {
		return nil, ErrAborted
	}
	return fm.selectedValues(), nil
}

// treeRow addresses one visible line: a node header (leaf == -1) or a leaf.
type treeRow struct {
	node int
	leaf int
}

type treeModel struct {
	title     string
	nodes     []TreeNode
	collapsed map[int]bool
	selected  map[string]bool
	rows      []treeRow
	cursor    int
	filter    string
	filtering bool
	accepted  bool
	aborted   bool
	hasDarkBg bool // terminal background, for the ink-accent cursor/checks
}

// These carry no accent colour, so they're background-independent. The cursor
// and check glyphs are the ink accent, resolved per render from hasDarkBg via
// accentStyle.
var (
	treeTitleStyle = lipgloss.NewStyle().Bold(true)
	treeNodeStyle  = lipgloss.NewStyle().Bold(true)
	treeDimStyle   = lipgloss.NewStyle().Faint(true)
)

func newTreeModel(title string, nodes []TreeNode) treeModel {
	m := treeModel{
		title:     title,
		nodes:     nodes,
		collapsed: map[int]bool{},
		selected:  map[string]bool{},
		hasDarkBg: true, // default to dotty's dark surface until detected
	}
	m.rebuildRows()
	return m
}

// rebuildRows recomputes the visible rows from the collapse state and filter.
// A non-empty filter fuzzy-matches leaf labels, hides empty nodes, and forces
// matching nodes open.
func (m *treeModel) rebuildRows() {
	m.rows = m.rows[:0]
	for ni, node := range m.nodes {
		leaves := node.Leaves
		if m.filter != "" {
			labels := make([]string, len(leaves))
			for i, l := range leaves {
				labels[i] = l.Label
			}
			matched := fuzzy.Find(m.filter, labels)
			if len(matched) == 0 {
				continue
			}
			kept := make([]int, len(matched))
			for i, match := range matched {
				kept[i] = match.Index
			}
			m.rows = append(m.rows, treeRow{node: ni, leaf: -1})
			for _, li := range kept {
				m.rows = append(m.rows, treeRow{node: ni, leaf: li})
			}
			continue
		}
		m.rows = append(m.rows, treeRow{node: ni, leaf: -1})
		if m.collapsed[ni] {
			continue
		}
		for li := range leaves {
			m.rows = append(m.rows, treeRow{node: ni, leaf: li})
		}
	}
	if m.cursor >= len(m.rows) {
		m.cursor = max(0, len(m.rows)-1)
	}
}

// Init implements tea.Model.
func (m treeModel) Init() tea.Cmd { return nil }

// Update implements tea.Model: navigation, collapse/expand, selection
// toggling, filtering, accept, and abort.
func (m treeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	if m.filtering {
		switch key.String() {
		case "esc":
			m.filtering = false
			m.filter = ""
			m.rebuildRows()
		case "enter":
			m.filtering = false
		case "backspace":
			if m.filter != "" {
				m.filter = m.filter[:len(m.filter)-1]
				m.rebuildRows()
			}
		case "ctrl+c":
			m.aborted = true
			return m, tea.Quit
		default:
			if key.Text != "" {
				m.filter += key.Text
				m.rebuildRows()
			}
		}
		return m, nil
	}

	switch key.String() {
	case "esc", "q", "ctrl+c":
		m.aborted = true
		return m, tea.Quit
	case "enter":
		m.accepted = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "left", "h":
		m.collapseAtCursor()
	case "right", "l":
		m.expandAtCursor()
	case "space":
		m.toggleAtCursor()
	case "/":
		m.filtering = true
	}
	return m, nil
}

func (m *treeModel) collapseAtCursor() {
	if len(m.rows) == 0 || m.filter != "" {
		return
	}
	row := m.rows[m.cursor]
	if row.leaf >= 0 {
		// On a leaf, jump to its parent node row.
		for i := m.cursor; i >= 0; i-- {
			if m.rows[i].node == row.node && m.rows[i].leaf == -1 {
				m.cursor = i
				break
			}
		}
		return
	}
	m.collapsed[row.node] = true
	m.rebuildRows()
}

func (m *treeModel) expandAtCursor() {
	if len(m.rows) == 0 || m.filter != "" {
		return
	}
	row := m.rows[m.cursor]
	if row.leaf == -1 {
		delete(m.collapsed, row.node)
		m.rebuildRows()
	}
}

// toggleAtCursor toggles a leaf, or all leaves of a node at once.
func (m *treeModel) toggleAtCursor() {
	if len(m.rows) == 0 {
		return
	}
	row := m.rows[m.cursor]
	node := m.nodes[row.node]
	if row.leaf >= 0 {
		v := node.Leaves[row.leaf].Value
		m.selected[v] = !m.selected[v]
		return
	}
	all := true
	for _, leaf := range node.Leaves {
		if !m.selected[leaf.Value] {
			all = false
			break
		}
	}
	for _, leaf := range node.Leaves {
		m.selected[leaf.Value] = !all
	}
}

func (m treeModel) selectedValues() []string {
	var values []string
	for _, node := range m.nodes {
		for _, leaf := range node.Leaves {
			if m.selected[leaf.Value] {
				values = append(values, leaf.Value)
			}
		}
	}
	return values
}

// View implements tea.Model.
func (m treeModel) View() tea.View {
	if m.accepted || m.aborted {
		return tea.NewView("")
	}
	var b strings.Builder
	accent := accentStyle(m.hasDarkBg)
	fmt.Fprintf(&b, "\n  %s\n", treeTitleStyle.Render(m.title))
	if m.filtering || m.filter != "" {
		fmt.Fprintf(&b, "  / %s\n", m.filter)
	}
	for i, row := range m.rows {
		cursor := "  "
		if i == m.cursor {
			cursor = accent.Render("❯ ")
		}
		if row.leaf == -1 {
			marker := "▾"
			if m.collapsed[row.node] && m.filter == "" {
				marker = "▸"
			}
			fmt.Fprintf(&b, "  %s%s %s\n", cursor, marker, treeNodeStyle.Render(m.nodes[row.node].Label))
			continue
		}
		leaf := m.nodes[row.node].Leaves[row.leaf]
		check := "[ ]"
		if m.selected[leaf.Value] {
			check = accent.Render("[x]")
		}
		fmt.Fprintf(&b, "  %s  %s %s\n", cursor, check, leaf.Label)
	}
	b.WriteString(treeDimStyle.Render("\n  space toggle · h/l collapse/expand · / filter · enter accept · esc cancel\n"))
	return tea.NewView(b.String())
}
