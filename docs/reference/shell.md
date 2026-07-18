<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

<!-- Source of truth: internal/scaffold/template/home/.config/{zsh,ghostty,git,oh-my-posh}/
     and home/.ssh/config — update this page when those templates change. -->

# Shell & terminal

Everything on this page ships with the `core` component of every scaffold — you
get it regardless of which addons and agents you pick. It is all themed
**cyberdream** end to end: Ghostty, the prompt, `ls` colours, and every addon
share one palette.

## zsh

dotty relocates zsh under XDG: a one-line `~/.zshenv` sets
`ZDOTDIR=$XDG_CONFIG_HOME/zsh`, and everything else lives in `.config/zsh/`. The
env file defines the XDG base dirs, pins Homebrew's global Brewfile to
`$XDG_DATA_HOME/homebrew/Brewfile` (with `HOMEBREW_REQUIRE_TAP_TRUST=1`), moves
Go's caches under XDG, wires `SSH_ASKPASS` to the dotty askpass applet (see
[Signing keys](../getting-started/signing.md)), and sources the active profile's
`env.zsh` — which is how per-profile values like `$DOTTY_WORKTREES` and agent
home relocations reach the shell.

Plugins load through [zinit](https://github.com/zdharma-continuum/zinit):

- **zsh-syntax-highlighting** and **zsh-autosuggestions**
- **fzf-tab** — tab completion becomes an fzf picker, with directory previews
  for `cd` and zoxide jumps
- Oh My Zsh **sudo** (double-press ++esc++ to prefix `sudo`) and
  **command-not-found** snippets

Integrations initialised at startup: [oh-my-posh](https://ohmyposh.dev/)
(prompt, `prompt.yaml`), fzf keybindings, **zoxide as `cd`**, direnv, and mise
(each guarded, so machines without a tool skip it). History keeps 5000
deduplicated, shared-across-sessions entries; `vivid` generates `LS_COLORS` from
the cyberdream theme.

Aliases:

| Alias                         | Expands to                                       |
| ----------------------------- | ------------------------------------------------ |
| `ls`, `ll`, `la`, `lla`, `lt` | `lsd` variants (list, long, all, long+all, tree) |
| `sts`                         | `dotty tmux new` — start/attach a repo session   |
| `lts`                         | `tmux list-session`                              |
| `kts`                         | `tmux kill-server`                               |
| `vim`, `vi`                   | `nvim` (when installed)                          |
| `y`                           | yazi file manager that `cd`s to where you quit   |

## Ghostty

`.config/ghostty/config` sets the cyberdream theme in both variants
(`theme = dark:cyberdream,light:cyberdream-light` follows the system
appearance), **Iosevka NF** at 15pt with font thickening, a hidden titlebar,
background opacity 0.95 with blur, Option-as-Alt, and a Display P3 colourspace.
It also maps the private-use codepoints `U+F4000–U+F47FF` to the **lobe-icons**
glyph font — the AI-brand glyphs that `dotty init` installs and that name the
agent windows in [tmux](tmux.md). Matching terminfo entries for `xterm-ghostty`
are linked into `~/.terminfo`.

## Git

`.config/git/config` is the shared baseline; identity and signing arrive via
includes so the repo itself never contains PII:

- **Behaviour**: `pull.ff only` + `pull.rebase true`, autoStash/autoSquash
  rebases, `init.defaultBranch main`, git-lfs wired, a commit template dir whose
  `prepare-commit-msg` hook appends a `Signed-off-by` trailer.
- **Stacked-branch workflow**: aliases onto `dotty git` — `git start`,
  `git append`, `git propose`, `git sync` (fetch trunk, prune merged layers,
  refresh PR maps, rebase+resign when diverged), `git stack` to pick a layer
  (stack status is `dotty git status` directly — git cannot alias over the
  `status` builtin), `git up` / `git down` for navigation, `git resign`, and
  `git browse` (opens the upstream forge, falling back to origin) —
  signature-preserving, trunk-based stacks with ff-merge (see
  [Signature-preserving stacks](../guides/stacks.md)).
- **Conditional includes by host**: GitHub remotes get `gh` as the credential
  helper; Codeberg/Forgejo remotes get Git Credential Manager with OAuth client
  IDs.
- **Identity include**: `~/.config/private/git/config` — your name/email,
  written once by `dotty init`, kept outside the repo (see
  [Where things live](layout.md)).
- **Per-profile signing include**:
  `~/.config/dotty/active-profile/git.gitconfig` — SSH signing via
  `dotty signing-key` on machines whose profile uses security keys; silently
  skipped elsewhere.
- **Worktrees include** (loaded last, so it wins): disables commit/tag signing
  inside agent worktrees — see
  [Agent worktrees & re-signing](../guides/worktrees.md).
- **Global ignore**: `commit.sh`, local Claude settings and plans, `.DS_Store`,
  `.env.*` (with `.env.dotty` kept), and the worktrees directory.

## SSH

With security keys enabled, `~/.ssh/config` contains a
`Match host * exec "dotty signing-key link"` rule — every connection first
refreshes the stable `~/.ssh/id_sk_current` symlink to the active profile's
allowed key — plus `IdentitiesOnly yes` and `IdentityAgent none` (no ssh-agent;
the key never leaves the YubiKey, and PIN prompts go through pinentry-mac).

## Themed addons

Each optional addon renders a small, cyberdream-skinned config:

- **btop** — resource monitor
- **k9s** — Kubernetes TUI (skin + aliases)
- **lazygit** — git TUI (also included by the nvim addon)
- **lsd** — the `ls` replacement behind the aliases above
- **yazi** — terminal file manager (theme + bat syntax theme)
- **vivid** — the `LS_COLORS` generator theme
