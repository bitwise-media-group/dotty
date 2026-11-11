<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

<!-- Source of truth: internal/scaffold/template/home/.config/tmux/conf/*.conf
     — update this page when those templates change. -->

# tmux

The `tmux` addon (`dotty init --addons=tmux,…`) renders an opinionated tmux
setup into your dotfiles repo at `.config/tmux/`: a thin `tmux.conf` that
sources `conf/plugins.conf`, `conf/keymaps.conf`, `conf/sidebar.conf`,
`conf/options.conf`, and `conf/theme.conf`, and bootstraps
[TPM](https://github.com/tmux-plugins/tpm) on first launch.

The prefix is ++ctrl+a++.

## Keybindings

All bindings below are pressed **after the prefix** unless marked _(root)_.

### Panes and windows

| Keys                          | Action                                         |
| ----------------------------- | ---------------------------------------------- |
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
| ++shift+a++ | OpenClaude  |
| ++shift+c++ | Claude Code |
| ++shift+g++ | Grok        |
| ++shift+o++ | OpenCode    |
| ++shift+x++ | Codex       |

OpenClaude starts with its saved provider configuration. Use its `/provider`
menu to sign in and switch providers within the session, including Anthropic,
ChatGPT/Codex OAuth, and xAI/Grok subscriptions; use `/model` to switch models.
See
[OpenClaude's subscription guide](https://openclaude.gitlawb.com/docs/providers/#subscriptions).

Selecting the `openclaude` agent also renders settings and the cyberdream theme
under `.config/openclaude/` and exports
`OPENCLAUDE_CONFIG_DIR="${XDG_CONFIG_HOME}/openclaude"`. Its settings reuse the
Claude Code template, including permissions, sandbox settings, worktree and tmux
hooks, status line, and the optional skills marketplace. The pinned Claude model
is omitted so OpenClaude can resolve a model for the chosen provider. The shared
instructions are available as `CLAUDE.md`; settings are per-profile, while
OpenClaude's credentials, sessions, and provider profiles stay separate from
Claude Code. OpenClaude normally uses `~/.openclaude` and falls back to
`CLAUDE_CONFIG_DIR` when `OPENCLAUDE_CONFIG_DIR` is unset, so the explicit
override also prevents accidentally sharing Claude Code's directory. See
[OpenClaude's configuration guide](https://openclaude.gitlawb.com/docs/configuration).

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

| Keys                                                       | Action                                                                                                                  |
| ---------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| ++backspace++ or ++f++                                     | Kill the current session                                                                                                |
| ++s++                                                      | Show and focus the [session sidebar](#the-session-sidebar); again from inside it, hide it (`choose-tree` without dotty) |
| ++u++                                                      | Open a URL from the pane via fzf (tmux-fzf-url)                                                                         |
| ++ctrl+h++ / ++ctrl+j++ / ++ctrl+k++ / ++ctrl+l++ _(root)_ | Move between tmux panes and Neovim splits (vim-tmux-navigator)                                                          |
| ++shift+enter++ _(root)_                                   | Sends ++ctrl+j++ — Claude Code's newline chord, worked around because tmux lacks kitty-keyboard-protocol support        |

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

| Status      | Rendered as             | Meaning                                            |
| ----------- | ----------------------- | -------------------------------------------------- |
| `attention` | red background, 🔔 bell | The agent needs an approval or answer              |
| `waiting`   | peach dot ●             | The agent finished its turn and is waiting for you |
| _(clear)_   | —                       | The agent is working, or idle                      |

The token is set by the internal `dotty tmux set-status` command, which every
scaffolded agent calls from its lifecycle hooks (Claude Code and Codex hooks,
Grok's `hooks/tmux-status.json`, OpenCode's `plugin/agent-tmux-status.js`).
Glance at the status bar to see which agent window needs you, or at the session
sidebar below for every session at once.

`set-status` only writes (and wakes the sidebar) when the status actually
changes: agents clear the token on every tool call, and those repeats are no-ops
that neither redraw the status bar nor wake anything.

## The session sidebar

`conf/sidebar.conf` adds a narrow pane on the right edge of the window you are
looking at, in every session, running
[`dotty tmux sidebar`](../cli/dotty_tmux_sidebar.md). It shows every session and
its windows as a tree with the agent status glyphs: 󰂚 needs attention, ● is
waiting. A collapsed session's badge counts its windows in the most urgent
state, and the session and window in view are highlighted.

Click a row to jump to that session or window. From the keyboard, move into the
sidebar with ++ctrl+l++ (vim-tmux-navigator) or ++s++, then:

| Keys                | Action                                                        |
| ------------------- | ------------------------------------------------------------- |
| ++j++ / ++k++       | Move down / up                                                |
| ++h++ / ++l++       | Collapse / expand a session                                   |
| ++n++ / ++shift+n++ | Next / previous window needing you — attention, then waiting  |
| ++slash++           | Filter by session or window name (++enter++ jumps to the hit) |
| ++enter++           | Jump to the session or window under the cursor                |
| ++esc++             | Hand focus back to the pane you came from                     |
| ++q++               | Hide the sidebar (as ++s++ does from inside it)               |

There is one sidebar for the whole tmux server, not one per session: tmux has no
pane that spans windows, so the sidebar's pane moves (`join-pane`) into the
window you switch to, leaving focus where it was. Two consequences:

- The panes of a window reflow once when the sidebar arrives. Neovim and the
  agents redraw on resize.
- With several terminals attached to the server, the sidebar shows in the one
  most recently switched.

The sidebar costs nothing while idle: it blocks on a tmux `wait-for` channel,
and the hooks that wake it (switching windows or sessions, windows opening,
closing or being renamed, agent status changes) run inside the tmux server
without forking. When a window's last other pane exits, the sidebar steps aside
so the window closes as it would without it.

The global `@dotty_sidebar` option (`on` by default) records whether the sidebar
shows. ++s++ and ++q++ flip it, and re-sourcing the config keeps your choice;
set it `off` in your own config to start hidden. Without `dotty` on `PATH`,
++s++ falls back to tmux's `choose-tree` and the hooks do nothing.

## Sessions with dotty

[`dotty tmux new`](../cli/dotty_tmux_new.md) fuzzy-picks a repository and
creates (or re-attaches to) a development session for it, with one window per
installed coding agent. The scaffolded zsh config aliases it to `sts`. Agent
worktree sessions created by
[`dotty git worktree start`](../guides/worktrees.md) appear grouped under their
parent repo in the Neovim session picker;
[`dotty git worktree remove`](../cli/dotty_git_worktree_remove.md) kills a
worktree's session along with the worktree (but never the session it runs in).
New sessions get the [session sidebar](#the-session-sidebar) as soon as you
attach or switch to them.
