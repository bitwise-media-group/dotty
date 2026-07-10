// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"errors"

	"charm.land/huh/v2"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// ErrNotInteractive is returned when a prompt is required but the streams are
// not attached to a terminal. Callers translate it into "pass --<flag>"
// guidance — a hidden prompt must never block a script or git.
var ErrNotInteractive = errors.New("interactive prompt required but no terminal is attached")

// ErrAborted is returned when the user backs out of a prompt (esc / ctrl+c).
var ErrAborted = errors.New("aborted")

// Option is one selectable entry: Label is what the user sees, Value is what
// the caller gets back.
type Option struct {
	Label string
	Value string
}

// maxPromptRows caps how many rows a picklist field renders. Without a cap
// huh sizes the viewport to the full option count, so a long list repaints
// hundreds of lines per frame and floods the terminal. Longer lists scroll
// inside the viewport, and fuzzy filtering still searches every option, so
// typing surfaces entries that have not rendered yet. Lists at or under the
// cap keep huh's auto height (no blank padding rows).
const maxPromptRows = 14

// Confirm asks a yes/no question. description may be empty.
func Confirm(ios cli.IOStreams, title, description string) (bool, error) {
	var ok bool
	field := huh.NewConfirm().Title(title).Description(description).Value(&ok)
	if err := runForm(ios, field); err != nil {
		return false, err
	}
	return ok, nil
}

// Input asks for a single line of text, re-prompting until validate accepts
// it. validate may be nil.
func Input(ios cli.IOStreams, title, placeholder string, validate func(string) error) (string, error) {
	var value string
	field := huh.NewInput().Title(title).Placeholder(placeholder).Value(&value)
	if validate != nil {
		field = field.Validate(validate)
	}
	if err := runForm(ios, field); err != nil {
		return "", err
	}
	return value, nil
}

// Password asks for a single secret line with masked input, re-prompting until
// validate accepts it. validate may be nil.
func Password(ios cli.IOStreams, title string, validate func(string) error) (string, error) {
	var value string
	field := huh.NewInput().Title(title).EchoMode(huh.EchoModePassword).Value(&value)
	if validate != nil {
		field = field.Validate(validate)
	}
	if err := runForm(ios, field); err != nil {
		return "", err
	}
	return value, nil
}

// MultiSelect presents a fuzzy-filterable checklist and returns the Values of
// the chosen options, in option order.
func MultiSelect(ios cli.IOStreams, title string, options []Option) ([]string, error) {
	var values []string
	field := huh.NewMultiSelect[string]().Title(title).Options(huhOptions(options)...).Filterable(true).Value(&values)
	if len(options) > maxPromptRows {
		field.Height(maxPromptRows)
	}
	if err := runForm(ios, field); err != nil {
		return nil, err
	}
	return values, nil
}

// Select presents a fuzzy-filterable picklist (huh filters on "/" and typing)
// and returns the chosen option's Value.
func Select(ios cli.IOStreams, title string, options []Option) (string, error) {
	var value string
	field := selectField(title, options, &value)
	if err := runForm(ios, field); err != nil {
		return "", err
	}
	return value, nil
}

// selectField builds the picklist field Select runs, split out so tests and
// benchmarks can exercise its sizing and rendering without a terminal.
func selectField(title string, options []Option, value *string) *huh.Select[string] {
	field := huh.NewSelect[string]().Title(title).Options(huhOptions(options)...).Filtering(true).Value(value)
	if len(options) > maxPromptRows {
		field.Height(maxPromptRows)
	}
	return field
}

// huhOptions converts dotty options to huh's option type.
func huhOptions(options []Option) []huh.Option[string] {
	huhOpts := make([]huh.Option[string], len(options))
	for i, o := range options {
		huhOpts[i] = huh.NewOption(o.Label, o.Value)
	}
	return huhOpts
}

// runForm wraps a single field in a themed form bound to the IOStreams,
// guarding against non-interactive streams and normalizing huh's abort error.
func runForm(ios cli.IOStreams, field huh.Field) error {
	if !ios.IsInteractive() {
		return ErrNotInteractive
	}
	form := huh.NewForm(huh.NewGroup(field)).
		WithTheme(Theme(detectDark(ios))).
		WithInput(ios.In).
		WithOutput(ios.ErrOut)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return ErrAborted
		}
		return err
	}
	return nil
}
