// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"github.com/spf13/cobra"
)

var tmuxCmd = &cobra.Command{
	Use:   "tmux <verb>",
	Short: "Tmux sessions for agent-driven development.",
	Long: `Start and attach tmux dev sessions laid out for coding agents: an editor
window with a small shell split, one window per installed agent (opencode,
grok, codex, claude), and a shell window, all named after the repository.

The session sidebar follows you between windows and sessions, showing every
session's windows with each agent's status, so you can jump to whichever one
needs you.`,
	Example: `  dotty tmux new
  dotty tmux new dotty
  dotty tmux sidebar toggle`,
}

func init() {
	rootCmd.AddCommand(tmuxCmd)
}
