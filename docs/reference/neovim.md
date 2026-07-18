<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

<!-- Source of truth: internal/scaffold/template/home/.config/nvim/
     — update this page when those templates change. -->

# Neovim

The `nvim` addon renders a complete Neovim setup into your repo at
`.config/nvim/`. It is built on [LazyVim](https://www.lazyvim.org/) with the
[lazy.nvim](https://github.com/folke/lazy.nvim) plugin manager, themed
cyberdream like the rest of the scaffold, and pins every plugin in
`lazy-lock.json` so a fresh machine reproduces the exact same editor. Selecting
the addon also pulls in the lazygit config.

## LazyVim extras

Enabled in `lua/config/lazy.lua`:

- **Coding**: blink (completion), luasnip, yanky, mini-surround
- **Debugging**: dap.core, dap.nlua
- **Formatting / linting**: prettier, eslint
- **Testing**: test.core (neotest)
- **Util**: octo (GitHub PRs/issues), mini-hipatterns, dot (dotfile filetypes)
- **Languages**: docker, dotnet, go, helm, json, markdown, python, sql,
  terraform, typescript, yaml

The update checker is disabled (updates are deliberate, via the lock file) and
~30 unused built-in plugins are removed from the runtime path for startup speed.

## Options

Highlights from `lua/config/options.lua`:

- **Picker/completion**: snacks as the LazyVim picker, `cmp = "auto"`.
- **Clipboard over SSH**: custom OSC 52 setup — yanks reach the local system
  clipboard even inside SSH + tmux, while pastes read the unnamed register to
  avoid the notorious multi-second hang.
- **Python LSP**: `ty` (`lazyvim_python_lsp = "ty"`).
- **Visual guides**: `colorcolumn = "80,120"`, `list` with custom listchars,
  single global statusline (`laststatus = 3`), no tabline, spell checking on.
- **Providers**: python3/perl/ruby/node providers disabled; `vim.loader`
  enabled.

## Keymaps

Custom maps on top of LazyVim's defaults (`lua/config/keymaps.lua`):

| Keys                                              | Action                                                          |
| ------------------------------------------------- | --------------------------------------------------------------- |
| ++tab++ / ++shift+tab++                           | Next / previous buffer                                          |
| ++less++ / ++greater++                            | Deindent / indent the current line                              |
| ++plus++ / ++minus++                              | Increment / decrement number under cursor                       |
| ++shift+u++                                       | Redo                                                            |
| ++ctrl+c++                                        | Yank the entire buffer                                          |
| ++ctrl+e++                                        | Select all                                                      |
| ++alt+s++                                         | Save without formatting                                         |
| ++"<leader>"+exclam++ / ++"<leader>"+at++         | Add / remove word from spelling dictionary                      |
| ++"<leader>"+f+s++                                | tmux session picker (see below)                                 |
| ++"<leader>"+f+x++                                | LuaSnip snippet picker                                          |
| ++"<leader>"+s+f++                                | Search & replace in buffer (grug-far)                           |
| ++ctrl+h++ / ++ctrl+j++ / ++ctrl+k++ / ++ctrl+l++ | Move across Neovim splits _and_ tmux panes (vim-tmux-navigator) |

## Custom plugins

`lua/plugins/` adds and reshapes:

- **cyberdream.nvim** — the colourscheme, loaded first, with a matching lualine
  theme.
- **auto-save** — saves on `InsertLeave` (500 ms debounce); toggle with
  ++"<leader>"+u+v++.
- **snacks** (customised) — hidden/ignored files visible in the explorer and
  pickers, and a bespoke **tmux session picker** on ++"<leader>"+f+s++: it renders
  repo sessions with their agent-worktree sessions nested beneath (worktree root
  discovered from `$DOTTY_WORKTREES` — see
  [Agent worktrees & re-signing](../guides/worktrees.md)), shows a git-status
  preview, and switches the tmux client on confirm.
- **grug-far** — project-wide search and replace.
- **vim-tmux-navigator** — the tmux-side counterpart lives in the
  [tmux config](tmux.md).
- **ghostty** — loads Ghostty's bundled Neovim runtime files.
- **blink** — completion without preselection.
- **sqlfluff** — SQL formatting and linting via conform + nvim-lint.

Autocmds strip trailing whitespace on save, stop comment continuation on new
lines, toggle relative/absolute line numbers with focus and mode, and keep
utility windows (lazy, mason, notify, …) unlisted and easy to close.

## Language tooling

`lua/plugins/lang/` configures LSPs and tools beyond the LazyVim extras:

| Language | Tooling                                              |
| -------- | ---------------------------------------------------- |
| Go       | gopls (semantic tokens, directory filters) + gofumpt |
| OpenTofu | tofu-ls + tflint LSP, trivy, `tofu fmt`              |
| Protobuf | buf LSP, buf lint/format                             |
| XML      | lemminx                                              |
| TOML     | tombi                                                |
| Jinja    | djlint (lint + format)                               |
| Markdown | mermaid CLI (`mmdc`) for diagram preview             |

## Updating plugins

`lazy-lock.json` pins all plugins. Updates are an explicit act: run
`:Lazy update` (or `:Lazy sync`), review, and commit the changed lock file to
your dotfiles repo — every other machine picks the same versions up on its next
`:Lazy restore`.
