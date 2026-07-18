<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Daily workflow

Setup is done. These are the loops you'll actually run day to day — each one is
two or three commands, with a guide behind it when you need more.

## Packages

```sh
dotty brewfile add jq          # install + record in the profile's Brewfile
git -C ~/Repos/dotfiles commit -am "feat: add jq" && git push
```

On your other machines: pull, then `dotty brewfile sync` to converge — mind that
[sync removes unlisted packages](../guides/brewfile.md).

## Dotfiles

Edit configs **in the repo** (they're symlinked, so changes apply live), then
commit. After adding new files, re-link and check:

```sh
dotty dotfiles link
dotty dotfiles status
```

## Secrets

```sh
dotty env add GITHUB_TOKEN
dotty env run -- gh api user
```

Secrets live in the macOS Keychain, never in files; `env run` injects them for
exactly one command. See [Credentials & the Keychain](../guides/credentials.md).

## Sessions

```sh
sts        # alias for: dotty tmux new
```

Fuzzy-pick a repo, get a tmux session with a window per coding agent, and watch
the status bar for the ● / 🔔 agent indicators. From Neovim, ++"<leader>"+f+s++
jumps between sessions and their agent worktrees.

## A second machine

1. [Install dotty](../install.md).
2. Clone your dotfiles repo.
3. `dotty init --repo <clone>` (or run it from inside) — the stored answers walk
   you through, and the machine adopts the profile's class.
4. If the machine belongs to a _different_ class:
   [`dotty profile activate`](../guides/profiles.md).

## Staying in sync

The repo is the sync channel — there is no daemon. Commit and push changes; on
other machines pull, then `dotty dotfiles link` and `dotty brewfile sync` as
needed. When an agent has been working in a worktree,
[`dotty git resign`](../guides/worktrees.md) before you merge.
