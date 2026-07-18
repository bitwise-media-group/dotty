<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

<!-- Source of truth: internal/scaffold/template/home/.config/tmux/conf/*.conf
     — update this page when those templates change. -->

# tmux

The `tmux` addon (`dotty init --addons=tmux,…`) renders an opinionated tmux
setup into your dotfiles repo at `.config/tmux/`: a thin `tmux.conf` that
sources `conf/plugins.conf`, `conf/keymaps.conf`, `conf/options.conf`, and
`conf/theme.conf`, and bootstraps [TPM](https://github.com/tmux-plugins/tpm) on
first launch.

The prefix is ++ctrl+a++.

## Keybindings

All bindings below are pressed **after the prefix** unless marked _(root)_.

### Panes and windows

| Keys                  | Action                                         |
| --------------------- | ---------------------------------------------- |
| ++bar++                       | Split horizontally, in the current pane's path |
| ++minus++                     | Split vertically, in the current pane's path   |
| ++ctrl+p++                    | Previous window                                |
| ++ctrl+n++                    | Next window                                    |
| ++h++ / ++j++ / ++k++ / ++l++ | Resize pane by 5 (repeatable — keep tapping)   |
| ++m++                         | Toggle pane zoom (repeatable)                  |

The stock ++percent++ and ++double-quote++ split bindings are unbound in favour
of ++bar++ and ++minus++.

### Coding agents

Each of these opens a new window at the current pane's path running the agent,
with a lobe-icons glyph in the window name — and displays a message instead if
the agent isn't installed:

| Keys        | Agent       |
| ----------- | ----------- |
| ++shift+c++ | Claude Code |
| ++shift+g++ | Grok        |
| ++shift+o++ | OpenCode    |
| ++shift+x++ | Codex       |

### Copy mode (vi keys)

`mode-keys` is `vi`. In copy mode:

| Keys  | Action          |
| ----- | --------------- |
| ++v++ | Begin selection |
| ++y++ | Copy selection  |

Mouse-drag selection does **not** exit copy mode (the default
`MouseDragEnd1Pane` binding is removed), so you can adjust a selection before
yanking.

### Sessions and utilities

| Keys                                                       | Action                                                                                                           |
| ---------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| ++backspace++ or ++f++                                     | Kill the current session                                                                                         |
| ++u++                                                      | Open a URL from the pane via fzf (tmux-fzf-url)                                                                  |
| ++ctrl+h++ / ++ctrl+j++ / ++ctrl+k++ / ++ctrl+l++ _(root)_ | Move between tmux panes and Neovim splits (vim-tmux-navigator)                                                   |
| ++shift+enter++ _(root)_                                   | Sends ++ctrl+j++ — Claude Code's newline chord, worked around because tmux lacks kitty-keyboard-protocol support |

## Options

Notable settings from `conf/options.conf`:

| Option                           | Value                 | Why                                                           |
| -------------------------------- | --------------------- | ------------------------------------------------------------- |
| `mouse`                          | `on`                  | Click panes, drag splits, wheel-scroll                        |
| `status-position`                | `top`                 | Status bar above the panes                                    |
| `base-index` / `pane-base-index` | `1`                   | Windows and panes number from 1                               |
| `renumber-windows`               | `1`                   | No gaps when windows close                                    |
| `escape-time`                    | `0`                   | No Esc delay (vim-friendly)                                   |
| `history-limit`                  | `50000`               | Deep scrollback                                               |
| `set-clipboard`                  | `on`                  | Yanks reach the system clipboard, including over SSH (OSC 52) |
| `extended-keys`                  | `on` (`xterm` format) | Modifier-rich chords pass through to apps                     |
| `allow-passthrough`              | `on`                  | Escape-sequence passthrough (images, OSC)                     |
| `focus-events`                   | `on`                  | Neovim autoread etc. work inside tmux                         |
| `aggressive-resize`              | `on`                  | Windows resize to the smallest _viewing_ client only          |
| `lock-after-time`                | `3600`                | Lock idle sessions after an hour                              |

The config also exports `CLAUDE_CODE_TMUX_TRUECOLOR` and
`CLAUDE_CODE_NO_FLICKER` so Claude Code renders correctly under tmux, and keeps
plugin state under `~/.local/share/tmux` (XDG, not `~/.tmux`).

## Plugins

Managed by TPM (auto-installed on first launch):

- **tmux-sensible** — uncontroversial baseline settings.
- **tmux-fzf-url** — fuzzy-pick and open URLs from the scrollback (++u++).
- **vim-tmux-navigator** — one set of
  ++ctrl+h++/++ctrl+j++/++ctrl+k++/++ctrl+l++ motions across tmux panes and
  Neovim splits.
- **tmux-nerd-font-window-name** — window names get Nerd Font icons for the
  running program.

## Theme and the agent status indicator

`conf/theme.conf` loads [catppuccin/tmux](https://github.com/catppuccin/tmux)
with a custom `cyberdream` flavour (matching Ghostty, Neovim, and the rest of
the scaffold). The status bar sits at the top: session name on the left (red
while the prefix is active), the current pane's command and zoom flag in the
centre, host and an SSH lock glyph on the right.

Each window's status also renders an `@agent_status` token:

| Status      | Rendered as            | Meaning                                            |
| ----------- | ---------------------- | -------------------------------------------------- |
| `attention` | red background, 🔔 bell | The agent needs an approval or answer              |
| `waiting`   | peach dot ●            | The agent finished its turn and is waiting for you |
| _(clear)_   | —                      | The agent is working, or idle                      |

The token is set by the internal `dotty tmux set-status` command, which every
scaffolded agent calls from its lifecycle hooks (Claude Code and Codex hooks,
Grok's `hooks/tmux-status.json`, OpenCode's `plugin/agent-tmux-status.js`).
Glance at the status bar to see which agent window needs you.

## Sessions with dotty

[`dotty tmux new`](../cli/dotty_tmux_new.md) fuzzy-picks a repository and
creates (or re-attaches to) a development session for it, with one window per
installed coding agent. The scaffolded zsh config aliases it to `sts`. Agent
worktree sessions created by [`dotty worktree start`](../guides/worktrees.md)
appear grouped under their parent repo in the Neovim session picker.
