// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

// renderField runs a field through a themed huh form sized like a terminal
// and returns one rendered frame.
func renderField(t *testing.T, field huh.Field) string {
	t.Helper()
	form := huh.NewForm(huh.NewGroup(field)).WithTheme(Theme(true))
	form.Init()
	m, _ := form.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m.(*huh.Form).View()
}

func serialOptions(n int) []Option {
	options := make([]Option, n)
	for i := range options {
		options[i] = Option{
			Label:    fmt.Sprintf("serial-%03d", i),
			Value:    fmt.Sprintf("%d", i),
			Selected: true,
		}
	}
	return options
}

// TestMultiSelectFieldShowsEveryOption pins the workaround for huh v2.0.3's
// MultiSelect viewport sizing, which lost the title's row from an auto-sized
// viewport and so always cut off the last option — a one-entry list rendered
// as an empty picklist.
func TestMultiSelectFieldShowsEveryOption(t *testing.T) {
	cases := []struct {
		name  string
		count int
	}{
		{"single option", 1},
		{"two options", 2},
		{"full window", maxPromptRows},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			options := serialOptions(c.count)
			var values []string
			view := renderField(t, multiSelectField("Allow which keys?", options, &values))
			for _, o := range options {
				if !strings.Contains(view, o.Label) {
					t.Errorf("view missing option %q:\n%s", o.Label, view)
				}
			}
		})
	}
}

// TestMultiSelectFieldWindowCap pins the other side of the explicit height:
// long lists still render at most maxPromptRows rows and scroll for the rest.
func TestMultiSelectFieldWindowCap(t *testing.T) {
	var values []string
	view := renderField(t, multiSelectField("Allow which keys?", serialOptions(maxPromptRows+5), &values))
	if rendered := strings.Count(view, "serial-"); rendered != maxPromptRows {
		t.Errorf("rendered %d options, want %d:\n%s", rendered, maxPromptRows, view)
	}
}
