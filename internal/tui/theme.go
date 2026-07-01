// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"image/color"
	"io"
	"os"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// Dotty's brand palette, taken straight from the design-system "theme-dotty"
// tokens (bitwise-media-group/design-system, tokens/themes.css): a warm surface
// with a yellow primary. These slots do NOT vary with the terminal background:
//
//   - colorPrimary is the yellow FILL (button backgrounds, cursor tiles); dark
//     ink (colorOnPrimary) always sits on it. Both stay put in light mode —
//     .theme-dotty-light leaves --dotty-yellow / --dotty-on-accent unchanged.
//   - The status colours come from the shared earthy set in tokens/colors.css
//     :root (--green-500 … --blue-500). Dark mode there re-points only the
//     primary/surface/text aliases, not these raw tokens, so they're constant
//     across light and dark too.
//
// The tokens that DO flip between the two surfaces live in palette below.
// lipgloss v2's Color is a constructor rather than a string type, so these are
// vars, not consts.
var (
	colorPrimary   = lipgloss.Color("#F5C518") // --dotty-yellow (fill)
	colorOnPrimary = lipgloss.Color("#15140F") // --dotty-on-accent (ink on fill)

	colorSuccess = lipgloss.Color("#5E8C4F") // --green-500 (moss)
	colorWarning = lipgloss.Color("#E0A030") // --amber-500 (honey)
	colorDanger  = lipgloss.Color("#C2452F") // --red-500 (brick)
	colorInfo    = lipgloss.Color("#3F7C8C") // --blue-500 (slate teal)
)

// palette holds the dotty tokens that differ between the dark (.theme-dotty)
// and light (.theme-dotty-light) surfaces. ink is the accent used as TEXT and
// strokes (--dotty-ink-accent): the yellow fill reads fine on the dark surface
// but is illegible as text on light paper, so it drops to a deep amber there.
type palette struct {
	ink   color.Color // --dotty-ink-accent (accent as text/strokes)
	fg    color.Color // --text-strong (--dotty-fg)
	muted color.Color // --text-muted (--dotty-muted)
	line  color.Color // --border-subtle (--dotty-line)
}

// paletteFor resolves the mode-dependent palette against a dark or light
// terminal background, picking each token's .theme-dotty / .theme-dotty-light
// value. lipgloss.LightDark takes (lightValue, darkValue).
func paletteFor(isDark bool) palette {
	pick := lipgloss.LightDark(isDark)
	return palette{
		ink:   pick(lipgloss.Color("#7E6100"), lipgloss.Color("#F5C518")),
		fg:    pick(lipgloss.Color("#211E14"), lipgloss.Color("#F4EFDD")),
		muted: pick(lipgloss.Color("#6F6748"), lipgloss.Color("#A39A77")),
		line:  pick(lipgloss.Color("#E6DECB"), lipgloss.Color("#34301F")),
	}
}

// detectDark reports whether the terminal behind ios paints on a dark
// background, so the palette can flip between .theme-dotty and its light
// variant. It queries the terminal synchronously (OSC 11, near-instant with a
// 2s cap); non-*os.File streams or an unanswered query fall back to dark,
// dotty's default surface. dotty detects this itself because huh v2 only
// queries the background from its spinner, never its forms, so huh's own isDark
// is always false. Callers must have confirmed ios.IsInteractive first.
func detectDark(ios cli.IOStreams) bool {
	in, ok := ios.In.(*os.File)
	if !ok {
		return true
	}
	out, ok := ios.ErrOut.(*os.File)
	if !ok {
		return true
	}
	return lipgloss.HasDarkBackground(in, out)
}

// accentStyle is the ink-accent foreground for dotty's raw bubbletea views (the
// tree and table cursors and checkmarks). huh forms get the same colour through
// themeDotty; these render outside huh, so they resolve it per background here.
func accentStyle(isDark bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(paletteFor(isDark).ink)
}

// Theme returns the huh theme every dotty form uses, so prompts look the same
// across commands. isDark selects the light or dark variant of the palette;
// callers pass detectDark(ios). huh's ThemeFunc receives its own isDark, but
// huh never detects the background for forms (see detectDark), so we ignore it
// and bake in the value dotty measured.
func Theme(isDark bool) huh.Theme {
	return huh.ThemeFunc(func(bool) *huh.Styles {
		return themeDotty(isDark)
	})
}

// themeDotty builds huh's Styles from dotty's brand palette. It starts from
// huh.ThemeBase for the structural styles (borders, prefixes, indicators) and
// recolours every accent slot. Accent TEXT slots (titles, selectors, cursors)
// use the ink accent, which flips to a deep amber on light; the yellow fill
// (the focused button) keeps colorPrimary with dark ink in both modes.
func themeDotty(isDark bool) *huh.Styles {
	t := huh.ThemeBase(isDark)
	p := paletteFor(isDark)

	t.Focused.Base = t.Focused.Base.BorderForeground(p.line)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(p.ink).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(p.ink).Bold(true).MarginBottom(1)
	t.Focused.Directory = t.Focused.Directory.Foreground(p.ink)
	t.Focused.Description = t.Focused.Description.Foreground(p.muted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(colorDanger)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(colorDanger)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(p.ink)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(p.ink)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(p.ink)
	t.Focused.Option = t.Focused.Option.Foreground(p.fg)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(p.ink)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(colorSuccess)
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(colorSuccess).SetString("✓ ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(p.muted).SetString("• ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(p.fg)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(colorOnPrimary).Background(colorPrimary).Bold(true)
	t.Focused.Next = t.Focused.FocusedButton
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(p.muted).Background(p.line)

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(p.ink)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(p.muted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(p.ink)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description
	return t
}

// The notice styles reuse the status palette so standalone messages match the
// forms. These are the shared earthy status colours, which the design system
// keeps constant across light and dark, so a single variant is correct in both
// — no per-background detection needed here.
var (
	successStyle = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(colorInfo)
	warnStyle    = lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
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
