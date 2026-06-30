// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"io"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// Theme returns the huh theme every dotty form uses, so prompts look the same
// across commands. ThemeFunc lets huh resolve the light/dark variants against
// the terminal background it detects when the form runs.
func Theme() huh.Theme {
	return huh.ThemeFunc(huh.ThemeCharm)
}

// lipgloss v2 dropped AdaptiveColor, so these accent colours are the dark-
// background variants of the former pairs. huh's own forms stay adaptive via
// Theme above; these only style the standalone notices and custom prompts,
// which are dark-terminal oriented.
var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#02BF87")).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#7571F9"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFB454")).Bold(true)
)

// Successf prints a styled success notice to ErrOut.
func Successf(ios cli.IOStreams, format string, a ...any) {
	notice(ios.ErrOut, successStyle, "✓", format, a...)
}

// Infof prints a styled informational notice to ErrOut.
func Infof(ios cli.IOStreams, format string, a ...any) {
	notice(ios.ErrOut, infoStyle, "•", format, a...)
}

// Warnf prints a styled warning notice to ErrOut.
func Warnf(ios cli.IOStreams, format string, a ...any) {
	notice(ios.ErrOut, warnStyle, "!", format, a...)
}

func notice(w io.Writer, style lipgloss.Style, glyph, format string, a ...any) {
	_, _ = fmt.Fprintf(w, "%s %s\n", style.Render(glyph), fmt.Sprintf(format, a...))
}
