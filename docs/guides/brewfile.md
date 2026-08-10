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
`profiles/<profile>/Brewfile` and is linked to where Homebrew's global bundle
commands expect it (`HOMEBREW_BUNDLE_FILE_GLOBAL`).

## Adding packages

```sh
dotty brewfile add ripgrep
dotty brewfile add --cask ghostty
dotty brewfile add bitwise-media-group/tap/dotty
```

[`dotty brewfile add`](../cli/dotty_brewfile_add.md) installs the package _and_
records it in the profile's Brewfile in one step. Entries the Brewfile already
lists are detected with brew's own parser (`brew bundle list`) and skipped
rather than duplicated — `brew bundle add` alone would append a second copy —
and the bundle is still installed so the machine converges. Note that casks
compare by their short token (brew's semantics), so `acme/tap/widget` counts as
present when a bare `cask "widget"` entry exists.

!!! note "Tapped names and `brew trust`"

    Homebrew v6 refuses to install from an untrusted third-party tap. For
    tapped names (and third-party taps themselves), dotty checks trust first
    (`brew trust --json`) and walks you through granting it, so `brewfile add`
    doesn't fail halfway. It also records `trusted: true` on the new Brewfile
    entry: [`dotty brewfile sync`](../cli/dotty_brewfile_sync.md) runs
    `brew bundle install --force-cleanup`, which resets Homebrew's trust store
    to exactly what the Brewfile declares — a grant that lives only in the
    store would be revoked on the next sync.

## Removing packages

```sh
dotty brewfile remove ripgrep
dotty brewfile remove --cask ghostty
dotty brewfile rm --tap          # no names: pick from a checklist
```

[`dotty brewfile remove`](../cli/dotty_brewfile_remove.md) deletes entries from
the profile's Brewfile (via `brew bundle remove`) and revokes any `brew trust`
grant a removed tapped name carried. It does **not** uninstall anything — the
package stays on the machine until
[`dotty brewfile sync`](../cli/dotty_brewfile_sync.md) removes what the Brewfile
no longer lists. Pass `--sync` to run that immediately.

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
