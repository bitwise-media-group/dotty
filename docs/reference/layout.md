<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Where things live

dotty splits state across three places with three different privacy levels: your
**dotfiles repository** (shareable), **public config** under `$XDG_CONFIG_HOME`
(safe to commit, safe to show), and **private data** under `$XDG_DATA_HOME`
(secrets and personal state, never in a repo). Knowing which is which is most of
the mental model.

## The repository

`dotty init` scaffolds (or adopts) a dotfiles repository — by default
`<repos-dir>/dotfiles`, recognised later by its `.dotty-version` marker:

```text
dotfiles/
├── .dotty-version            # marks this repo as dotty-managed
├── Brewfile                  # composed from brewfile.d for the profile
├── brewfile.d/               # per-component Brewfile fragments
├── profiles/                 # one directory per profile
│   ├── personal/
│   │   ├── profile.json      # metadata + the profile's stored init answers
│   │   ├── Brewfile          # the profile's package set
│   │   ├── env.zsh           # profile-varying files (also git.gitconfig,
│   │   │                     #   worktrees.gitconfig)
│   │   └── home/             # profile-varying $HOME entries
│   └── work/
└── home/                     # everything below maps into $HOME
    ├── .config/
    │   └── zsh/  git/  ghostty/  tmux/  nvim/  …
    └── .ssh/config           # when security keys are enabled
```

[`dotty dotfiles link`](../cli/dotty_dotfiles_link.md) symlinks the `home` tree
into `$HOME` using symlink-farm folded links;
[`dotty dotfiles status`](../cli/dotty_dotfiles_status.md) shows what is linked,
missing, or conflicting.

Profiles travel **with the repo** — they describe machine classes (personal,
work), not individual machines. See [Profiles](../guides/profiles.md).

## Public config — `$XDG_CONFIG_HOME/dotty`

| Path                             | Purpose                                                                           |
| -------------------------------- | --------------------------------------------------------------------------------- |
| `~/.config/dotty/<profile>/`     | Symlinks into the repo's `profiles/<name>` directories                            |
| `~/.config/dotty/active-profile` | Symlink to the active profile — the **only** machine-local piece of profile state |

Nothing here is secret: it is symlinks into your dotfiles repo plus one more.
Retargeting `active-profile`
([`dotty profile activate`](../cli/dotty_profile_activate.md)) atomically swaps
every per-profile rendered file at once.

## Private data — `$XDG_DATA_HOME/dotty`

Created `0700`, contents `0600`. This is personal, per-machine state that must
never land in a repository:

| Path                                        | Purpose                                                                                                                                        |
| ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `~/.local/share/dotty/`                     | Signing-key stubs (the non-secret halves of YubiKey-resident SSH keys)                                                                         |
| `~/.local/share/dotty/backups/<timestamp>/` | Timestamped backups taken when linking with `--on-conflict=backup` — recover with [`dotty dotfiles restore`](../cli/dotty_dotfiles_restore.md) |
| `~/.local/share/dotty/dotty-ssh-askpass`    | The askpass applet symlink that routes OpenSSH PIN prompts to pinentry-mac                                                                     |

Security-key serial→alias mappings also live on the private side — key serials
identify your hardware and stay out of the public repo.

## PII — `~/.config/private/git/config`

Your git `user.name` / `user.email` (and the `commit.gpgSign` default) are
written **once** by `dotty init` to `~/.config/private/git/config`, which the
shared git config includes. It is never overwritten on re-runs and never part of
the dotfiles repo — so the repo can be public without leaking who you are. If
you keep a second, private dotfiles repo, link this file from there.

## Keychain

Secrets managed by [`dotty env`](../guides/credentials.md) don't live on disk at
all — they are items in the macOS login Keychain.
