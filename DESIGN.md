Represents a GoLang CLI that enables common utilities for operating a
terminal-driven workflow and dotfiles with the following commands. Use cobra,
huh, and bubbletea. Make the CLI visually pleasing when prompting the user.

# Ergonomics

ALL commands should take the following form:

```text
dotty <noun> <verb>
```

ALL state should be saved in `$XDG_DATA_HOME/dotty/<area>`

Flags should allow either a space or equals separator. For example:

```sh
dotty something --key=value
dotty something --key hello
```

For any command that proxies to another program, ensure that `--help` is always
handled by `dotty`.

# File Layout

```text
./go.mod
./cmd/main.go # main entry point and rootcmd
./cmd/<noun>.go # area command
./cmd/<noun>_<verb>.go # area verb command

./internal/<area>/... # contains any helper functions, structs, tools, etc. that are required to support exactly one area
./internal/cli/... # contains common cli-related options like IOStreams, Exec helpers, Path helpers, etc.
./internal/tui/... # contains supporting huh/bubbletea code for selections, prompts, etc.
```

If there are cross-area concerns, create a new package in the ./internal with a
clear but concise package name.

# Architecture

Commands should be a var within the package and the flags / subcommands attached
in the `init()`, for example:

```go
type RootFlags struct {
  ...
}

var rootFlags = NewRootFlags()
var rootCmd = &cobra.Command{}

init() {
    rootCmd.PersistentFlags().DurationVar(&rootArgs.timeout, "timeout", 30*time.Second, "timeout for the cli")
    rootCmd.PersistentFlags().BoolVar(&rootArgs.verbose, "verbose", false, "enable verbose logging")
    rootCmd.AddCmd(someAreaCmd)
}
```

The root command should have a `--profile=<name>` global flag, which is only
used by the `brewfile` command below, but will be used for other purposes in the
future.

# System Profiles

Comand: profile

Manage profiles that are unique configurations to use across machines. These
include things such as an oh-my-posh prompt, a brew bundle Brewfile, and
terminal themes. For now it will only manage a brew bundle file, but more will
come later.

## Create a profile

Command: new

Create a system-level profile that can be copied across machines. A profile
creates `profiles/<name>` in the dotfiles repository, exposed on the machine as
a `${XDG_CONFIG_HOME}/dotty/<name>` symlink into it. Once a profile is created,
ask the user if they want to activate it. Upon confirmation, activate the
profile using the command for `dotty profile activate` defined below.

```text
dotty profile new [--name=<name>] [--description=<description>] [--activate]
```

## Activate a profile

Command: activate

Activate an existing profile. If no profile name is specified, present a
fuzzy-finding picklist to allow the user to select a profile. If the name flag
is specified and the profile does not exist, ask the user if they wish to create
a new profile. Upon confirmation, invoke the cmd for
`dotty profile new --name=<name> --activate` to activate it.

To activate a profile, update the `${XDG_CONFIG_HOME}/dotty/active-profile`
symlink to point to the profile path. Determine if a Brewfile exists in that
path. If no brewfile exists, execute the cmd for `dotty brewfile dump` defined
below.

```text
dotty profile activate [--name=<name>]
```

# Command: Security Keys

Command: security-key Alias: sk

Manage hardware security keys.

## Add a security key alias

Command: add

Add a named alias for a signing key based on its serial number. The
serial-to-alias mapping is profile content: it lives in the profile directory
(security-keys.json), travels with the dotfiles repository, and activating
another profile swaps it — only the key stubs themselves stay in the private
data directory. If no name is specified, prompt the user for a name. Ensure that
the provided name is unique. Offer to open an editor (default EDITOR) to provide
a description if none is provided. The user may also skip this step.

```text
dotty security-key [--serial=<serial-number>] add [--name=<name>] [--description=<description>]
```

## Remove a security key alias

Command: remove Aliases: rm

Remove a named alias for a signing key based on its serial number. If no name is
provided, provide a fuzzy-finding multi-select picklist to allow the user to
remove multiple aliases. Since a key may have more than one alias. Since the
user may have more than one alias for a given key, use a tree-view that can
collapse and expand by serial number with the names underneath.

```text
dotty security-key [--serial=<serial-number>] remove [--name=<name>]
```

## Allow security keys for a profile

Command: allow

Restrict a profile to specific security keys. Once a profile has an allowlist,
its machines refuse every other key for signing, linking, enrollment, and import
— so personal keys are never used against work devices, and vice versa. The list
applies to the active profile unless the global `--profile` names another. Like
the alias mapping, it is profile content: it lives in the profile's
profile.json, travels with the dotfiles repository, and activating another
profile swaps it — serials identify hardware a machine class owns, not a
machine. Arguments are serials or aliases; without arguments, an interactive
picklist offers the known and connected keys with the current selection
preselected. `dotty init --allowed-serials=<a,b>` (or the wizard's restriction
question, asked after key enrollment) seeds the list.

```text
dotty [--profile=<name>] security-key allow [<serial>|<alias>...]
```

## Disallow security keys for a profile

Command: disallow

Remove security keys from a profile's allowlist. Removing the last entry lifts
the restriction — the profile allows every key again. Without arguments, an
interactive picklist offers the currently allowed keys.

```text
dotty [--profile=<name>] security-key disallow [<serial>|<alias>...]
```

# Signing Keys

Command: signing-key Aliases: ssh-key

Create, list, get, and use a signing key to sign payloads, such as git commits,
git tags, or files.

If no serial/name is provided in any command and only one security key is
currently plugged into the machine, use that key. If more than one security key
is plugged into the machine, activate the touch on the yubikey and prompt the
user to touch an yubikey to select it.

When the active profile carries a security-key allowlist
(`dotty security-key allow`), every key-using path — get, link, sign, new,
import, trust — refuses keys outside it, naming the profile and the escape
hatch.

## Create a new signing key

Command: new

This command creates a new SSH resident signing key using the currently plugged
in security key. If more than one security key is plugged in and a serial number
is not provided, then the user should be prompted to touch a signing key to
identify which one to use (have the signing keys activate a touch-required
activity).

The default key type should be ed25519.

The key should be generated with the below:

```sh
 ssh-keygen \
  -t "${type}"-sk \
  -f "${XDG_DATA_HOME}/dotty/security-key/<serial>/id_ed25519_sk_${user}" \
  -O resident \
  -O verify-required \
  -O no-touch-required \
  -O "application=ssh:${user}" \
  -O "user=${user}" \
  -C "${user}"
```

```text
dotty signing-key new [--security-key=[<serial-number>|<name>]] [--username=<username>] [--type=[ed25519, edcsa]]
```

## Get a signing key

Command: get

Get the value of the private key stub and public key for the specified username
from the identified security key. This should locate the key from the path
specified in the above keygen.

```text
dotty signing-key get [--security-key=[<serial-number>|<name>]] [--username=<username>]
```

## List signing keys

Command: list

List all of the security keys in a fuzzy-finding and selectable table for all
currently plugged in yubikeys, optionally restricted to the specified username.
The table should include the security key serial number, aliases, key type, and
username. Upon selection of a row, print the value of the private key stub and
public key. Escape should exit without printing anything. Do not require the
user to identify a security key if multiple keys are plugged in and no serial is
provided. This is the security-key/signing-key command where this is true.

```text
dotty signing-key list [--security-key=[<serial-number>|<name>]] [--username=<username>]
```

## Sign using a signing key

Command: sign

A proxy command to ssh-keygen to sign a payload with the selected signing key.
Primarily intended to be used by git via the gpg.ssh.defaultKeyCommand and
gpg.ssh.program config values.

```text
dotty signing-key sign [--security-key=[<serial-number>|<name>]] [--username=<username>]
```

## Trust a signing key

Command: trust

Append the signing keys on the currently plugged-in security keys to the OpenSSH
allowed_signers file, so git can verify the commits and tags they sign. Each
entry pairs the committer email (`git config user.email`) with the key's
algorithm and blob. The file is `gpg.ssh.allowedSignersFile` from git config,
falling back to `~/.ssh/allowed_signers`; existing entries are preserved and a
key already trusted for that email is left alone, so it is safe to re-run. The
`new` command runs this automatically after enrolling a key.

```text
dotty signing-key trust [--security-key=[<serial-number>|<name>]] [--username=<username>] [--path=<file>]
```

# System Profiles

Command: profile

Create a system-level profile that can be copied across machines.

# Brewfile Manipulation

Command: brewfile Aliases: brew

Manage a homebrew bundle Brewfile to maintain reproducible brew configurations
on and across systems.

## Add an entry to the brewfile

Command: add

Adds a brew cask, formula, or other supported brew types to the specified
brewfile. If the name of the brew contains more than one forward slash (/), then
determine if the brew is currently trusted via
`brew trust [--formula | --cask | --tap] --json v1`. If it is not trusted,
prompt the user to ask if they want to trust it before proceeding. If they
confirm, add the trust, and then the brew. This only applies to taps, casks, and
formulas. If no type is specified, the default is formulas. If other types are
specified, pass the arguments to `brew bundle add` as-is. The flow is:

```text
brew trust [--formula | --cask | --tap] <name>
brew bundle add [--formula | --cask | --tap] <name>
brew bundle install --file=<profile-path>/Brewfile
```

If a profile argument is specified, use the Brewfile in the profile path;
otherwise, use the symlinked path. If the path is not available, print an error.

```text
dotty [--profile=<profile>] brewfile add [--tap | --cask | --formula] <name> [...]
```

## Upgrade all brews

Upgrade all brews in the brewfile. This is akin to running:

```sh
brew bundle install --file=${XDG_CONFIG_HOME}/dotty/profile/Brewfile --upgrade
```

```text
dotty [--profile=<profile>] brewfile upgrade
```

## Sync bundles

Synchronise the machine with whatever is in the brewfile. If any brews would be
removed, prompt the user to confirm unless the force flag is set. This is akin
to running:

```sh
brew bundle install --file=${XDG_CONFIG_HOME}/dotty/profile/Brewfile --force --force-cleanup --upgrade --zap
```

```text
dotty [--profile=<profile>] brewfile sync [--force]
```

## Dump existing brews

Dumps any currently installed brews into the brewfile. This should only dump
mas, formula, cask, and flatpack unless the `--all` flag is set. This is akin to
running:

```sh
brew bundle dump --mas --flatpack --formulae --casks
```

```text
dotty [--profile=<profile>] brewfile dump [--all]
```

## Edit the brewfile

Opens the brewfile in the default editor

```text
dotty [--profile=<profile>] brewfile edit [--sync | --upgrade]
```

# Generic Credentials

Command: env

Store generic credentials in the operating system keychain and inject them into
templates and processes — the way the 1Password CLI does, but with no external
service. Credentials are grouped into namespaces; each namespace is a single
keychain item under the service name `dotty:<namespace>`, which isolates groups
of secrets that belong together. Every verb takes a `--namespace` flag (default
`default`).

Keychain access is platform-specific and lives behind an interface in
GOOS-tagged files; macOS shells out to `security`, with a stub on other
platforms until a Linux backend (e.g. `secret-tool`) is added. References use
the form `{{ dotty://<namespace>/<key> }}`, or a bare `{{ <key> }}` resolved
against `--namespace`.

## Add a credential

Command: add

Store a credential under KEY in the namespace. With a terminal attached the
value is read from a hidden prompt; when input is piped, it is read from stdin.
The value is never taken from a flag, so it stays out of shell history and the
process list.

With `--in-file`, KEY is omitted and a `.env` file is captured instead — the
inverse of `use`: every `KEY=value` assignment is stored in the namespace and
its value is replaced with a `{{ dotty://<namespace>/KEY }}` reference. The
result is written to `--out-file`, which defaults to `--in-file`; replacing an
existing file is confirmed first. Blank lines, comments, empty values, and
values that are already references are left untouched.

```text
dotty env [--namespace=<ns>] add <KEY>
dotty env [--namespace=<ns>] add --in-file=<file> [--out-file=<file>]
```

## Remove a credential

Command: remove Aliases: rm

Remove one credential by KEY, the whole namespace with `--all`, or pick several
from a filterable checklist when no KEY is given. Removing the last credential
also removes the namespace's keychain item.

```text
dotty env [--namespace=<ns>] remove [<KEY>] [--all]
```

## List credentials

Command: list Aliases: ls

Print the key names in the namespace, one per line, sorted. Values are never
printed.

```text
dotty env [--namespace=<ns>] list
```

## Get a credential

Command: get

Print a single credential value to stdout, like `op read`. The argument is
either a KEY in `--namespace` or a full `dotty://<namespace>/<key>` reference. A
trailing newline is printed unless `--no-newline`.

```text
dotty env [--namespace=<ns>] get <KEY | dotty://<namespace>/<key>> [--no-newline]
```

## Inject into a template

Command: use

Replace every reference in a template with its value, like `op inject`. The
template is read from `--in-file` or stdin and written to `--out-file` (created
with 0600) or stdout. An unknown or malformed reference is an error. With
neither `--namespace` nor `--in-file` and nothing piped in, the template
defaults to a `.env.dotty` in the working directory; a missing file is an error
with usage.

```text
dotty env [--namespace=<ns>] use [--in-file=<file>] [--out-file=<file>]
```

## Run with credentials in the environment

Command: run

Launch a command with every credential in the namespace exported as an
environment variable, like `op run`. dotty parses its own `--namespace` and
`--in-file` (and `--help`); everything after `--` is the command and its
arguments, passed through untouched. The command inherits the terminal, and
dotty exits with its exit code.

With `--in-file`, the environment is built from a `.env` template instead of the
whole namespace: every reference is resolved from the keychain and every plain
`KEY=value` assignment is passed through — like `use`, but the secrets go
straight to the process and are never written to disk. With neither
`--namespace` nor `--in-file`, the template defaults to a `.env.dotty` in the
working directory; a missing file is an error with usage.

```text
dotty env [--namespace=<ns>] run [--in-file=<file>] -- <command> [args...]
```

# Command: Init

Command: init

`init` (with `docs`, one of two sanctioned top-level verbs — it sets up the
whole machine, not one noun) scaffolds a net-new dotfiles repository from a
template embedded in the binary, driven by an interactive wizard. Every question
mirrors a flag so the command also runs non-interactively; prompts only fill in
what flags left unset, and nothing touches the filesystem until the user
confirms a summary of what will happen.

The wizard starts with the profile: when this machine (or the enclosing
repository) already has profiles, a picklist offers them plus "create a new
profile"; the global `--profile` or `--profile-name` selects one directly. A
profile that already has answers seeds every question's default with them —
re-running init, from inside the repository or anywhere after linking, walks the
same interview with the stored choices preselected, so a machine class can be
extended (or trimmed) later; non-interactive re-runs take the stored answers
as-is. For a new profile the wizard asks for the repositories directory (default
`~/Repos`) and the dotfiles repository path (default the enclosing repository
when run from inside one — recognized by its `.dotty-version` marker, which
records the dotty release that rendered it — else `<repos>/dotfiles`), both with
tab-completable suggestions. The paths persist portably in the profile: the
repositories directory home-relative, the repository relative to it, and
rendered shell files use `${HOME}` so no machine-specific prefix enters the
repository. The wizard also asks for a profile name when creating one (machine
name by default), whether to seed the Brewfile from the installed packages,
optional add-ons (nvim, btop, k9s, lazygit, lsd, tmux, yazi), and coding agents
(claude-code, codex, opencode, antigravity, grok). Once at least one agent is
selected, init offers the bitwise skills marketplace; choosing it wires the
marketplace into every selected agent that supports one. ghostty, oh-my-posh,
vivid, zsh, and git config are always included.

With agents selected, init also asks whether to harden them. Hardening mirrors
Claude Code's confinement into every selected agent's native config: sandboxed
writes limited to the repositories directory and tool caches, denied reads of
credential paths (~/.ssh, ~/.aws, ~/.gnupg, .env files), and ask-first
permissions — claude's sandbox/permissions blocks, codex's Seatbelt
workspace-write plus a PreToolUse deny hook, grok's sandbox profile and
permission rules plus the same hook, and opencode's permission matrix.
Unhardened agents keep only theme, hooks, and marketplace config. The choice is
per profile: a work class can run hardened while a personal one does not.

init then asks whether the machine class uses security keys. Answering yes
renders the profile's signing config (gpg/ssh via dotty), the `~/.ssh/config`
that signs and authenticates through `dotty signing-key link`, adds ykman and
pinentry-mac to the Brewfile, creates the `dotty-ssh-askpass` applet symlink in
the data directory (OpenSSH PIN prompts route through it to pinentry-mac, which
caches the YubiKey PIN), and offers to import an existing resident-key stub or
enroll a new key — the same flows as `dotty signing-key import` and `new`.
Separately, when `~/.config/private/git/config` does not exist, init asks for
the git identity and writes it there with `gpgSign` matching the security-key
answer; the file is PII, lives outside both the repository and the profile, and
is never overwritten.

On macOS, init finishes with the system questions: a picklist of curated
`defaults write` groups (keyboard, menu bar, trackpad, finder, screenshots,
software update, spaces, dock, animations, gpg keychain — everything
preselected), a wallpaper chosen from `~/.local/share/wallpapers` when that
directory has images (dotty distributes no wallpapers; the private repo supplies
them), and smart-card (PIV) login enforcement — off unless chosen, confirmed
with a lock-out warning, and applied through sudo. The selections persist in the
profile and re-apply on a re-run; the tweaks run last and best-effort, so a
declined sudo never unwinds a completed init.

Profiles are shared through the repository and activated per machine: one
dotfiles repo serves every machine, and a profile (personal, work) captures how
a class of machines differs — a `profile.json` (metadata plus the wizard answers
in one document), the composed Brewfile, a `home/` tree holding every
`$HOME`-relative file whose content varies by profile (paths like the
repositories directory, agent sandbox roots, marketplace enablement, signing
config), and loose files like `env.zsh` and the git includes at the profile
root. The profile's `env.zsh` also relocates every selected agent under XDG
(`CODEX_HOME` with `CODEX_SQLITE_HOME` pointed at XDG data, `CLAUDE_CONFIG_DIR`
with `CLAUDE_CODE_PLUGIN_CACHE_DIR` pointed at XDG cache, `GROK_HOME`), so agent
config lives in `~/.config/<agent>` like every other tool while runtime state
and caches stay out of the dotfiles-linked directories. The agent worktree
location is a profile setting too (`--worktrees`, default the repo-relative
`.worktrees`, or an absolute path for one shared root): it lands in the shared
gitignore when repo-relative, feeds the hardened agents' sandbox grants so
worktrees never prompt, renders the per-profile git include that disables
commit/tag signing inside linked worktrees (git matches `includeIf gitdir:`
against the resolved `.git/worktrees/<name>` path, so the blanket pattern covers
them wherever they live), is exported as `DOTTY_WORKTREES` for tooling like the
nvim session picker, and will drive `dotty worktree`. All of it lives under
`profiles/<name>` in the repo, with `${XDG_CONFIG_HOME}/dotty/<name>` a symlink
into it. Shared files never carry a profile-specific value; they reach profile
values only through the `${XDG_CONFIG_HOME}/dotty/active-profile` symlink — the
only real machine-local state — so `dotty profile activate` retargets everything
at once (e.g. `~/.config/dotty/active-profile/env.zsh`).

After confirmation, init renders the selected template components (shared files
into the repo, profile-varying files into `profiles/<name>`), composes the
Brewfile, runs `git init` and stages everything (the first commit is left to the
user so it can be signed), links the repository's `home` tree into `$HOME` plus
the profile-varying files through active-profile, activates the profile, and
downloads the pinned lobe-icons glyph font into the user font directory (a
warning, never a failure, when offline). Re-running init against an existing
repository and profile asks the same questions with the stored answers as the
defaults; keeping them re-renders and re-links idempotently — which is also how
a second machine of the same class adopts a freshly cloned repo, either run from
inside the clone or with `--repo` naming it. Against a legacy-layout repository,
re-running init first migrates it in place: profiles are lifted to top-level
`profiles/<name>` directories, each per-profile `render/` dissolves into its
profile root, `dotty.json` and `profile.json` merge into a single
`profile.json`, and the tree of `$HOME`-relative entries is renamed to `home/`.

```text
dotty init [--repo=<dir>] [--repos-dir=<dir>] [--profile-name=<name>]
           [--addons=<a,b>] [--agents=<a,b>] [--dump-brews] [--marketplace]
           [--harden] [--security-keys] [--git-name=<name>] [--git-email=<email>]
           [--allowed-serials=<a,b>] [--worktrees=<dir>]
           [--macos-defaults=<a,b>] [--wallpaper=<image>] [--piv]
           [--on-conflict=(backup|adopt|skip|fail)] [--yes]
```

# Command: Dotfiles

Command: dotfiles

Operate on the dotfiles repository init generated (or any repository with the
same layout: a `home/` tree of `$HOME`-relative entries plus a `profiles/`
directory of per-profile `profile.json` documents).

`link` symlinks the repository's `home` tree into `$HOME` as a symlink farm:
whole files and directories are linked folded, existing real directories are
descended into, and stale symlinks are replaced. A real file in the way is a
conflict resolved per `--on-conflict` (or interactively): `backup` moves it
under `$XDG_DATA_HOME/dotty/backups/<timestamp>/` mirroring its absolute path,
`adopt` moves it into the repository so the user's copy wins, `skip` leaves it,
`fail` aborts. Adoption is never offered for machine-generated destinations
(per-profile files, profile directories) — those back up instead, so a stale
live copy can never overwrite a fresh render.

`status` reports the plan without changing anything. `restore` copies a backup
set back over the links — every file dotty ever displaced stays recoverable.

```text
dotty dotfiles link [--repo=<dir>] [--on-conflict=(backup|adopt|skip|fail)]
dotty dotfiles status [--repo=<dir>]
dotty dotfiles restore [--timestamp=<ts>]
```

# Command: Worktree

Command: worktree

The agent worktree lifecycle. `start [repo] [suffix]` creates (or reuses) a
worktree for repo on an `agent/<repo>-<suffix>` branch at the configured
worktree location and prints its path — stdout is the result, so hooks and
scripts capture it. repo falls back to `$CLAUDE_PROJECT_DIR`, then the enclosing
repository; suffix falls back to the WorktreeCreate hook JSON on stdin
(`{"name": ...}`), then a UTC timestamp. `end [path]` reports uncommitted and
unpushed work, kills the tmux session named after the worktree, removes the
worktree, and deletes its branch — but only an `agent/*` branch, so manually
checked-out branches survive; path falls back to the WorktreeRemove hook JSON
(`{"worktree_path": ...}`), and an already-gone worktree exits quietly.

The location comes from the active profile (`DOTTY_WORKTREES`, set by
`dotty init --worktrees`): repo-relative by default, or one absolute shared
root. Names are tmux-safe (dots and other disallowed characters encode to
dashes), and the claude template wires WorktreeCreate/WorktreeRemove hooks at
these verbs. Inside the worktrees, commit/tag signing is off via the profile's
worktrees.gitconfig; re-sign with `dotty git resign`.

```text
dotty worktree start [repo] [suffix]
dotty worktree end [path]
```

# Command: git (stacks + navigation)

Command: git

Beyond `resign`, git verbs cover **signature-preserving stacks** for fork-only
repos that land with ff-merge:

- **start / append / propose / sync** — local lineage (git config
  `dotty.stack.*` / `dotty.branch.*`), trunk-based PRs (all base `main`,
  cumulative diffs), PR body stack maps, merged-layer cleanup (default delete
  local+origin), and prompt-to-rebase+resign when diverged (`sync --continue` /
  `--abort` around conflicts).
- **status** — print the current stack versus trunk (not plain `git status`).
  When the branch has no config lineage but local tips form an obvious chain of
  at least three nodes (trunk + two feature branches), lineage is discovered and
  saved.
- **up / down / switch** — navigate the current stack (switch is a fuzzy
  picklist, also reachable as `stack`).
- **browse** — open the upstream (else origin) forge homepage.

```text
dotty git start <branch>
dotty git append <branch>
dotty git status
dotty git propose [--all]
dotty git sync [--continue|--abort] [--yes]
dotty git up [num]
dotty git down [num]
dotty git switch    # alias: stack
dotty git browse
dotty git resign …
```
