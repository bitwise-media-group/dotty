<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Initialise a dotfiles repo

[`dotty init`](../cli/dotty_init.md) creates a dotfiles repository from the
template embedded in dotty and sets the machine up around it. The order is
deliberate: **every question comes before the first write**, and nothing at all
is written until you confirm a summary. After confirmation it renders the
repository, stages it with git (the first commit is left for you to
[sign](signing.md)), links the home tree into `$HOME`, activates the profile,
writes your private git identity, enrols security keys, installs the glyph font,
and applies macOS defaults last.

## Run the wizard

=== "Interactive"

    ```sh
    dotty init
    ```

    The wizard asks each question below in turn and ends with a summary;
    ++esc++ backs out at any point with nothing written.

=== "Non-interactive"

    ```sh
    dotty init \
      --profile-name=personal \
      --addons=tmux,nvim,lsd \
      --agents=claude-code,codex \
      --harden \
      --security-keys \
      --git-name="Ada Lovelace" --git-email=ada@example.com \
      --macos-defaults=keyboard,finder,dock \
      --yes
    ```

    Flags answer questions up front; `--yes` skips the confirmation
    summary. See [`dotty init`](../cli/dotty_init.md) for the full list.

## The decisions it asks for

### Where things go

Repositories directory (default `~/Repos`) and the repo path (default
`<repos-dir>/dotfiles`). To adopt an **existing** repository — say, a fresh
clone on a second machine — run `init` from inside it or point `--repo` at it;
dotty recognises its own repos by the `.dotty-version` marker. Re-running
`init` on a repository with the legacy layout migrates it in place.

### Profile name

The machine class this machine belongs to — `personal`, `work` — not the
machine's name. Answers are stored in the profile, so re-running `init` later
walks the same interview with your previous answers as defaults. See
[Profiles](../guides/profiles.md).

### Addons

Optional tools, each rendering its config into the repo: `nvim`, `btop`, `k9s`,
`lazygit`, `lsd`, `tmux`, `yazi`. ghostty, zsh, oh-my-posh, vivid, and git
config are always included. The [reference section](../reference/tmux.md)
documents what each config contains.

### Coding agents

`claude-code`, `codex`, `opencode`, `grok`, `antigravity` — plus whether to
**harden** them and whether to add the bitwise skills marketplace. Covered on
the [next page but one](agents.md).

### Git identity

Your name and email go to `~/.config/private/git/config` — outside the repo,
written only if it doesn't already exist, so the repo itself never contains PII.
Whether commits sign by default follows your security-keys answer.

### Security keys

Whether this machine class signs with hardware keys, and optionally which key
serials the profile allows (`--allowed-serials`). The [signing page](signing.md)
walks the enrolment that follows.

### Agent worktrees

Where sandboxed agents' git worktrees live: a directory name inside each repo
(default `.worktrees`) or one absolute shared root. See
[Agent worktrees & re-signing](../guides/worktrees.md).

### macOS

Which [defaults groups](../reference/macos-defaults.md) to apply, an optional
wallpaper from `~/.local/share/wallpapers`, and optional system-wide smart-card
(PIV) login enforcement.

### Conflicts

What to do when a real file already exists where a symlink should go:

!!! tip "The default, `backup`, is the safe one"

    Existing files are moved to a timestamped set under
    `$XDG_DATA_HOME/dotty/backups/` before linking, and
    [`dotty dotfiles restore`](../cli/dotty_dotfiles_restore.md) puts them
    back wholesale. `adopt` pulls the existing file's contents into the
    repo instead; `skip` leaves it alone; `fail` stops.

## What gets created

```text
dotfiles/
├── .dotty-version
├── Brewfile              # composed from brewfile.d fragments
├── brewfile.d/
├── profiles/<profile>/   # profile.json (answers), Brewfile, env.zsh, home/
└── home/                 # linked into $HOME
    └── .config/{zsh,git,ghostty,tmux,nvim,claude,…}/
```

[Where things live](../reference/layout.md) has the full map, including what
stays _out_ of the repo.

## Verify

```sh
dotty dotfiles status
```

[`status`](../cli/dotty_dotfiles_status.md) shows every link the repo defines
and its state — the plan view you'll also use after future edits.

!!! note "Your repo is staged, not committed"

    init deliberately stops short of the first commit so it can be
    **signed** with your hardware key. That's the next step.

[Next: Signing keys & first commit →](signing.md)
