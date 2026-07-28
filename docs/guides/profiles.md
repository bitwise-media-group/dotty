<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Profiles

A profile is a **machine class**, not a machine: `personal` and `work` are
profiles; your laptop is not. Profiles live inside the dotfiles repository
(`profiles/<name>/`) and travel with it — clone the repo on a new machine,
activate the right profile, and the machine adopts that class's package set,
keys policy, and rendered config.

## Anatomy

Each profile directory contains:

```text
profiles/work/
├── profile.json         # metadata + the stored init answers (addons, agents, …)
├── Brewfile             # the profile's package set
├── env.zsh              # per-profile environment (DOTTY_WORKTREES, agent homes)
├── git.gitconfig        # signing config, when this class uses security keys
├── worktrees.gitconfig  # signing-off include for agent worktrees
└── home/                # profile-varying $HOME entries, linked like the repo's
```

`profile.json` doubles as the answer store: re-running
[`dotty init`](../cli/dotty_init.md) with a profile selected seeds every wizard
question with that profile's previous answers.

The **only machine-local state** is one symlink:
`~/.config/dotty/active-profile` → the active profile's directory. Shared config
(git, zsh) references files _through_ that symlink — for example
`~/.config/dotty/active-profile/git.gitconfig` — so switching profiles
atomically swaps every profile-varying value at once, with no re-templating.

## Creating a profile

```sh
dotty profile new --name=work --description="employer machines"
```

[`dotty profile new`](../cli/dotty_profile_new.md) is
[`dotty init`](../cli/dotty_init.md) with the profile name settled in advance —
an empty profile directory is a machine class that configures nothing, so the
verb walks the same interview, renders `profiles/work/` into the repository,
links the home tree, and leaves the new profile active. Every init flag works
here too, so `--addons`, `--agents` and `--yes` create one unattended.

The name has to be free: re-rendering a profile that exists is `dotty init`, and
switching to it is `dotty profile activate`.

Create a new profile when machines genuinely differ in _policy_ — package set,
security keys, hardening, employer constraints. Two machines that should behave
identically belong to the same profile.

## Switching

```sh
dotty profile activate            # fuzzy-pick from the repo's profiles
dotty profile activate --name=work
```

[`dotty profile activate`](../cli/dotty_profile_activate.md) retargets the
`active-profile` symlink and, if the profile has no Brewfile yet, dumps the
current machine's packages as a starting point. After switching classes on a
machine, follow with [`dotty brewfile sync`](brewfile.md) to make the installed
packages match the new profile.

```mermaid
flowchart LR
    subgraph repo [dotfiles repo]
        P1[profiles/personal/]
        P2[profiles/work/]
    end
    A[~/.config/dotty/active-profile] -- symlink --> P2
    G[git config include] --> A
    Z[zsh env source] --> A
```

## Taking stock

```sh
dotty profile list          # every profile, * on the active one
dotty profile get           # the active profile in detail
dotty profile get work
dotty profile get work --format=json
```

[`dotty profile list`](../cli/dotty_profile_list.md) is the inventory — name,
description, creation date. [`dotty profile get`](../cli/dotty_profile_get.md)
adds what only this machine knows: the profile directory, the repository
directory behind it, whether it is active, and its Brewfile size. Named without
an argument, both work on the active profile, and the global `--profile` picks
another. `--format=json` prints `profile.json` verbatim, so the stored init
answers come with it.

## Deleting

```sh
dotty profile delete work
dotty profile delete --activate=personal    # retiring the active profile
```

[`dotty profile delete`](../cli/dotty_profile_delete.md) removes both halves of
a profile: the `~/.config/dotty/<name>` link _and_ the `profiles/<name>`
directory it resolves to. Removing only the link would delete nothing lasting —
the content is in the repository, and the next `dotty dotfiles link` would put
the link back. Commit the repository deletion afterwards.

Two things it will not do. The last profile cannot go, because a machine is
always of some class. Neither can the active one, unless you name a replacement
for it to activate first — `--activate`, or the picklist dotty offers — so the
`active-profile` symlink that git and zsh read through never dangles.

## Per-profile security

Two security controls are profile content, so they swap on activation:

- **Security-key allowlist** —
  [`dotty security-key allow`](../cli/dotty_security-key_allow.md) restricts
  which hardware keys the profile may use; every signing-key operation checks
  it. A work profile can be limited to employer-issued keys.
- **Signing config** — the profile's `git.gitconfig` turns SSH commit signing on
  (or not) per class; machines whose profile skips security keys sign nothing
  and git silently skips the missing include.
