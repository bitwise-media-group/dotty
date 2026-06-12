// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// Theme returns the huh theme every dotty form uses, so prompts look the same
// across commands.
func Theme() *huh.Theme {
	return huh.ThemeCharm()
}

var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"})
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#FF6F00", Dark: "#FFB454"}).Bold(true)
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
	fmt.Fprintf(w, "%s %s\n", style.Render(glyph), fmt.Sprintf(format, a...))
}
