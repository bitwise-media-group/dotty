// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// keypress builds the key message a typed rune produces, as huh's own tests do.
func keypress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: string(r), Code: r, ShiftedCode: r})
}

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

// TestSelectFieldCapsHeight pins the sizing rule: long lists render a bounded
// viewport (scrolling and filtering reach the rest), short lists keep huh's
// auto height so they show no blank padding rows.
func TestSelectFieldCapsHeight(t *testing.T) {
	tests := []struct {
		name    string
		options int
		maxRows int
	}{
		{name: "long list capped", options: 150, maxRows: maxPromptRows},
		{name: "short list uncapped", options: 3, maxRows: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var value string
			field := selectField("pick", benchOptions(tt.options), &value)
			if got := lipgloss.Height(field.View()); got > tt.maxRows {
				t.Errorf("rendered height = %d rows, want <= %d", got, tt.maxRows)
			}
		})
	}
}

// TestSelectFieldFilterReachesUnrendered pins the cap's contract: an option
// far beyond the rendered viewport is still reachable by typing a filter.
func TestSelectFieldFilterReachesUnrendered(t *testing.T) {
	var value string
	options := benchOptions(150)
	field := selectField("pick", options, &value)
	field.Focus()
	if strings.Contains(field.View(), options[149].Label) {
		t.Fatalf("option 149 already rendered; the fixture no longer exercises the cap")
	}

	for _, r := range "149" { // Filtering(true) starts in filter mode: typing filters
		field.Update(keypress(r))
	}
	if !strings.Contains(field.View(), options[149].Label) {
		t.Errorf("filtering for 149 did not surface %q in the viewport", options[149].Label)
	}
}

// BenchmarkSelectViewCapped measures rendering one frame of the picklist as
// Select builds it (viewport capped at maxPromptRows).
func BenchmarkSelectViewCapped(b *testing.B) {
	var value string
	field := selectField("pick", benchOptions(150), &value)
	b.ResetTimer()
	for b.Loop() {
		_ = field.View()
	}
}

// BenchmarkSelectViewUncapped measures the same frame without the height cap
// — huh sizes the viewport to all 150 options — quantifying what the cap
// saves per frame.
func BenchmarkSelectViewUncapped(b *testing.B) {
	var value string
	field := huh.NewSelect[string]().Title("pick").Options(huhOptions(benchOptions(150))...).
		Filtering(true).Value(&value)
	b.ResetTimer()
	for b.Loop() {
		_ = field.View()
	}
}
