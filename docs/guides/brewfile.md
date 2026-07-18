<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Brewfile workflow

dotty treats Homebrew as declarative: the profile's Brewfile is the source of
truth, and machines converge on it. Add packages through dotty so they land in
the repo, commit, and let every other machine in the class pick them up.

## How the Brewfile is composed

At scaffold time, `dotty init` assembles the profile's Brewfile from fragments
in the repo's `brewfile.d/` — one per selected component (`core.Brewfile`, one
per addon, one per agent, one for security keys) — plus, with `--dump-brews`,
whatever is already installed on the machine. The result lives at
`profiles/<profile>/Brewfile` and is linked to where Homebrew's global
bundle commands expect it (`HOMEBREW_BUNDLE_FILE_GLOBAL`).

## Adding packages

```sh
dotty brewfile add ripgrep
dotty brewfile add --cask ghostty
dotty brewfile add bitwise-media-group/tap/dotty
```

[`dotty brewfile add`](../cli/dotty_brewfile_add.md) installs the package _and_
records it in the profile's Brewfile in one step.

!!! note "Tapped names and `brew trust`"

    Homebrew v6 refuses to install from an untrusted third-party tap. For
    tapped names, dotty checks trust first (`brew trust --json`) and walks
    you through granting it, so `brewfile add` doesn't fail halfway.

## Snapshotting a machine

[`dotty brewfile dump`](../cli/dotty_brewfile_dump.md) writes the machine's
currently installed packages into the profile's Brewfile — useful when adopting
an existing machine whose software grew organically. `--all` includes
everything, not just top-level requests.

## Syncing

```sh
dotty brewfile sync
```

[`dotty brewfile sync`](../cli/dotty_brewfile_sync.md) makes the machine match
the Brewfile — installing what's listed and **removing what isn't**.

!!! danger "sync removes what the Brewfile doesn't list"

    Sync runs `brew bundle` with force, cleanup, and zap semantics: casks
    and formulae absent from the Brewfile are uninstalled, zap included.
    Run [`dotty brewfile dump`](../cli/dotty_brewfile_dump.md) (or review
    `brew bundle cleanup` output) first on a machine with packages you
    haven't recorded yet.

## Upgrading and hand-editing

- [`dotty brewfile upgrade`](../cli/dotty_brewfile_upgrade.md) upgrades the
  installed packages the Brewfile pins.
- [`dotty brewfile edit`](../cli/dotty_brewfile_edit.md) opens the profile's
  Brewfile in `$EDITOR`; pass `--sync` or `--upgrade` to apply the result
  immediately.

## The loop across machines

1. `dotty brewfile add <pkg>` on machine A — installed and recorded.
2. Commit and push the dotfiles repo.
3. On machine B: pull, then `dotty brewfile sync` — B converges on the same set.
