<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Packages workflow

dotty treats packages as declarative: the profile's mise directory is the source
of truth, and machines converge on it. Add packages through dotty so they land
in the repo, commit, and let every other machine in the class pick them up —
with the same locked versions.

## How a profile declares packages

A profile carries a whole mise global-config directory, which dotty links to
`~/.config/mise` through the active-profile symlink:

```text
profiles/<profile>/mise/
├── config.toml      # yours: [settings] lockfile, [bootstrap.brew] adopt, and the packages you add
├── conf.d/          # one fragment per selected component, rendered by dotty init
│   ├── core.toml    # fnox and age live here
│   ├── addon-nvim.toml
│   └── feature-security-keys.toml
└── mise.lock        # written by mise for every platform; commit it
```

Two kinds of entry live there:

- **Tools** (`[tools]`) — full backend ids such as `aqua:sharkdp/bat`,
  `github:owner/repo`, `npm:prettier`, `pipx:ruff`, or a mise core tool like
  `go` or `node`. mise installs them itself and records the exact version,
  download URL, and checksum for every platform in `mise.lock`, so macOS and
  Linux machines get byte-identical builds from one committed file.
- **Bootstrap packages** (`[bootstrap.packages]`) — `brew:git`,
  `brew-cask:ghostty`, `mas:497799835`, `flatpak:…`, `apt:…`. mise pours these
  into the Homebrew prefix (no Homebrew install required — kegs stay
  `brew`-compatible) or the OS package manager, at their latest version. They
  are for what has no static build (git, curl, zsh, the native libraries yazi
  previews need) and for GUI apps and fonts.

`dotty init` renders the fragments under `conf.d/` from the components you
select and prunes them when a component is deselected. `config.toml` is rendered
once from the template and then belongs to you: `dotty packages add`,
`mise use -g`, and hand edits all write there, and a re-run of init never resets
it. A machine that already had `~/.config/mise` when init first ran finds its
config and lockfile seeded into the profile.

!!! note "Spell the backend"

    Registry shorthand is refused: `git` resolves to git-chglog and `flux` to
    flux-operator. Pass `aqua:owner/repo`, `github:owner/repo`, `brew:name`,
    and so on; `mise registry <name>` lists the candidates for a name.

## Adding packages

```sh
dotty packages add aqua:sharkdp/bat aqua:jqlang/jq
dotty packages add brew-cask:raycast
dotty packages add github:bitwise-media-group/evolve
```

[`dotty packages add`](../cli/dotty_packages_add.md) records the ids in the
profile's `config.toml` _and_ installs them in one step: tools through
`mise use --global` (followed by `mise lock --global`, so the commit that adds
the tool carries its lock for every platform), bootstrap packages through
`mise bootstrap packages use --global`. Ids the profile already declares — in
`config.toml` or a component fragment — are skipped rather than duplicated, and
the profile is still installed so the machine converges.

## Removing packages

```sh
dotty packages remove aqua:sharkdp/bat brew-cask:raycast
dotty packages rm                       # no ids: pick from a checklist
dotty packages remove --sync brew:colima
```

[`dotty packages remove`](../cli/dotty_packages_remove.md) drops entries from
`config.toml` — tools through `mise unuse --global`, bootstrap packages by
removing the line, since mise has no `unuse` for them. An entry a component
fragment declares is refused: deselect the component with `dotty init` instead.
Nothing is uninstalled — the package stays on the machine until
[`dotty packages sync`](../cli/dotty_packages_sync.md) removes what the profile
no longer declares. Pass `--sync` to run that immediately.

## Snapshotting a machine

[`dotty packages import`](../cli/dotty_packages_import.md) records the Homebrew
formulae installed on the machine as `brew:` bootstrap packages — useful when
adopting an existing machine whose software grew organically. `--all` includes
dependencies, not just what was installed on request. Entries the profile
already declares are skipped, and new ones land under a `# installed packages`
comment, so the import is idempotent.

`--brewfile <path>` converts a Brewfile instead: formulae the components now
provide as locked tools are dropped, formulae mise cannot pour get their locked
stand-ins (`kubectl`, `terraform`, `flux`, `awscli`, `yt-dlp`, `actions-up`),
other formulae become `brew:` packages (with the taps they need — mise pours a
tapped formula only when the tap publishes API metadata, so those come with a
warning), casks become `brew-cask:` packages restricted to macOS (the agent CLIs
and the bitwise casks become their tools instead), and entry types mise has no
equivalent for (`vscode`, `krew`) are reported. A profile from before packages
moved to mise still carries its `Brewfile`; `dotty init` converts it once and
leaves the file for you to delete.

## Syncing

```sh
dotty packages sync
```

[`dotty packages sync`](../cli/dotty_packages_sync.md) makes the machine match
the profile: refreshes `mise.lock` for every platform, installs the locked tools
and the bootstrap packages that are missing, prunes tool versions nothing
references, and **removes bootstrap packages the profile no longer declares**.
Locking comes first, so a tool a component added since the last sync installs
from a resolved, checksummed entry.

!!! danger "sync removes what the profile doesn't declare"

    Before removing anything, sync runs `mise bootstrap packages prune
    --dry-run`, shows the list, and asks — `--force` skips the question.
    Run [`dotty packages import`](../cli/dotty_packages_import.md) first on a
    machine with formulae you haven't recorded yet. Cask removal is
    conservative: mise only removes casks it installed itself, so an app
    installed by hand may stay put and needs `brew uninstall --cask` (or a
    trip to `/Applications`) if you want it gone.

Tools are different: a tool version the profile no longer references is
mise-owned and invisible to the rest of the machine, so it is pruned without
asking.

## Upgrading and hand-editing

- [`dotty packages upgrade`](../cli/dotty_packages_upgrade.md) moves every tool
  and bootstrap package to its newest version and refreshes `mise.lock` for
  every platform, so the new versions travel with the profile.
- [`dotty packages edit`](../cli/dotty_packages_edit.md) opens the profile's
  `config.toml` in `$EDITOR`; pass `--sync` or `--upgrade` to apply the result
  immediately. After a hand edit that adds tools,
  [`dotty packages lock`](../cli/dotty_packages_lock.md) refreshes the lockfile.
- [`dotty packages status`](../cli/dotty_packages_status.md) lists the tools
  with their installed versions and the bootstrap packages with their state.

## The loop across machines

1. `dotty packages add <id>` on machine A — installed, recorded, locked.
2. Commit and push the dotfiles repo (`config.toml` and `mise.lock` together).
3. On machine B: pull, then `dotty packages sync` — B converges on the same set
   at the same versions, on macOS or Linux.

## What stays on Homebrew

Bootstrap packages track "latest", as the Brewfile did; anything that must pin
has to be a `[tools]` entry. Auto-updating apps (Ghostty, Raycast, the Claude
and ChatGPT desktop apps, Tailscale, …) are adopted as installed when their
content matches the current cask artifact — a drifted app may be downloaded
again on a fresh apply. `brew-cask:` entries carry `os = "macos"`, so on Linux
the fonts they provide are not installed. Homebrew itself is no longer needed —
mise pours bottles and casks directly — but a machine that has it keeps working:
the kegs are the same, and `brew` still sees them.
