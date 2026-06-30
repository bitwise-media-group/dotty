// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package tui

import (
	"errors"
	"fmt"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/bitwise-media-group/dotty/internal/cli"
)

// ErrTimeout is returned when a polled wait expires before completing.
var ErrTimeout = errors.New("timed out")

// RunPoll shows a spinner with title and hint while calling poll every
// interval until poll reports done (nil return), poll errors, the timeout
// expires (ErrTimeout), or the user presses esc (ErrAborted). Results travel
// through the poll closure.
func RunPoll(ios cli.IOStreams, title, hint string, interval, timeout time.Duration, poll func() (bool, error)) error {
	if !ios.IsInteractive() {
		return ErrNotInteractive
	}
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	m := pollModel{spinner: sp, title: title, hint: hint, interval: interval, remaining: timeout, poll: poll}
	p := tea.NewProgram(m, tea.WithInput(ios.In), tea.WithOutput(ios.ErrOut))
	final, err := p.Run()
	if err != nil {
		return fmt.Errorf("run poll UI: %w", err)
	}
	return final.(pollModel).result
}

type pollTickMsg struct{}

type pollModel struct {
	spinner   spinner.Model
	title     string
	hint      string
	interval  time.Duration
	remaining time.Duration
	poll      func() (bool, error)
	result    error
	done      bool
}

// Init starts the spinner and the poll cadence.
func (m pollModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.tick())
}

func (m pollModel) tick() tea.Cmd {
	return tea.Tick(m.interval, func(time.Time) tea.Msg { return pollTickMsg{} })
}

// Update drives the poll loop: esc aborts, each tick polls once and counts
// down the timeout.
func (m pollModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+c", "q":
			m.result = ErrAborted
			m.done = true
			return m, tea.Quit
		}
	case pollTickMsg:
		done, err := m.poll()
		switch {
		case err != nil:
			m.result = err
			m.done = true
			return m, tea.Quit
		case done:
			m.done = true
			return m, tea.Quit
		}
		m.remaining -= m.interval
		if m.remaining <= 0 {
			m.result = ErrTimeout
			m.done = true
			return m, tea.Quit
		}
		return m, m.tick()
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View renders the spinner line and hint.
func (m pollModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(fmt.Sprintf("\n  %s %s\n  %s\n", m.spinner.View(), m.title, infoStyle.Render(m.hint)))
}
