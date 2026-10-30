// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"errors"
	"slices"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
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
// the caller gets back. Selected preselects the entry in MultiSelect, for
// picklists that default to everything on. Locked keeps a MultiSelect entry
// selected: it starts ticked and cannot be unticked.
type Option struct {
	Label    string
	Value    string
	Selected bool
	Locked   bool
}

// maxPromptRows caps how many rows a picklist renders (the FuzzySelect window
// and the MultiSelect viewport). Without a cap huh sizes a static-options
// field to the full option count, so a long list repaints hundreds of lines
// per frame and floods the terminal. Longer lists scroll inside the window,
// and filtering still searches every option, so typing surfaces entries that
// have not rendered yet.
const maxPromptRows = 14

// Confirm asks a yes/no question defaulting to No. description may be empty.
func Confirm(ios cli.IOStreams, title, description string) (bool, error) {
	return ConfirmDefault(ios, title, description, false)
}

// ConfirmDefault is Confirm with the preselected answer chosen by the caller
// — re-runs seed prompts with previously stored answers.
func ConfirmDefault(ios cli.IOStreams, title, description string, def bool) (bool, error) {
	ok := def
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

// InputSuggest is Input with tab-completable suggestions — paths, mostly, so
// the user picks instead of typing. validate may be nil.
func InputSuggest(ios cli.IOStreams, title, placeholder string, suggestions []string,
	validate func(string) error) (string, error) {
	var value string
	field := huh.NewInput().Title(title).Placeholder(placeholder).Suggestions(suggestions).Value(&value)
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
// the chosen options, in option order. Locked options are always among them;
// a checklist carrying one is not filterable (see lockedMultiSelect).
func MultiSelect(ios cli.IOStreams, title string, options []Option) ([]string, error) {
	var values []string
	if err := runForm(ios, multiSelectField(title, options, &values)); err != nil {
		return nil, err
	}
	return values, nil
}

// multiSelectField builds the checklist field with an explicit height: huh
// v2.0.3 sizes an auto-height MultiSelect viewport to the option rows and
// then subtracts the title row, cutting off the last option — a one-entry
// list renders empty. An explicit height is treated as the field total, so
// the extra row covers the title and every option gets a row.
func multiSelectField(title string, options []Option, values *[]string) huh.Field {
	var locked []string
	for _, o := range options {
		if o.Locked {
			locked = append(locked, o.Value)
		}
	}
	field := huh.NewMultiSelect[string]().Title(title).Options(huhOptions(options)...).
		Filterable(len(locked) == 0).Value(values)
	field.Height(min(len(options), maxPromptRows) + 1)
	if len(locked) == 0 {
		return field
	}
	return &lockedMultiSelect{MultiSelect: field, values: values, locked: locked,
		toggle: huh.NewDefaultKeyMap().MultiSelect.Toggle}
}

// lockedMultiSelect keeps its locked values selected, which huh v2.0.3
// cannot express: a toggle on a locked row is swallowed, and anything else
// that unticks one (select none) is undone by re-ticking through the value
// binding. The re-tick reaches huh's displayed rows only while they share
// storage with its options, which filtering breaks — hence no filter.
type lockedMultiSelect struct {
	*huh.MultiSelect[string]
	values *[]string
	locked []string
	toggle key.Binding
}

// Update handles msg like the wrapped field, then restores the locked values.
func (m *lockedMultiSelect) Update(msg tea.Msg) (huh.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, m.toggle) {
		if hovered, ok := m.Hovered(); ok && slices.Contains(m.locked, hovered) {
			return m, nil
		}
	}
	_, cmd := m.MultiSelect.Update(msg)
	var missing []string
	for _, v := range m.locked {
		if !slices.Contains(*m.values, v) {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		*m.values = append(*m.values, missing...)
		m.Value(m.values)
	}
	return m, cmd
}

// WithKeyMap applies k to the wrapped field and tracks its toggle binding.
func (m *lockedMultiSelect) WithKeyMap(k *huh.KeyMap) huh.Field {
	m.toggle = k.MultiSelect.Toggle
	m.MultiSelect.WithKeyMap(k)
	return m
}

// huhOptions converts dotty options to huh's option type.
func huhOptions(options []Option) []huh.Option[string] {
	huhOpts := make([]huh.Option[string], len(options))
	for i, o := range options {
		huhOpts[i] = huh.NewOption(o.Label, o.Value).Selected(o.Selected || o.Locked)
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
