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
creates a path under `${XDG_CONFIG_HOME}/dotty/<name>`. Once a profile is
created, ask the user if they want to activate it. Upon confirmation, activate
the profile using the command for `dotty profile activate` defined below.

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

Add a named alias for a signing key based on its serial number. This should be
retained in JSON or YAML in the data directory. If no name is specified, prompt
the user for a name. Ensure that the provided name is unique. Offer to open an
editor (default EDITOR) to provide a description if none is provided. The user
may also skip this step.

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

# Signing Keys

Command: signing-key Aliases: ssh-key

Create, list, get, and use a signing key to sign payloads, such as git commits,
git tags, or files.

If no serial/name is provided in any command and only one security key is
currently plugged into the machine, use that key. If more than one security key
is plugged into the machine, activate the touch on the yubikey and prompt the
user to touch an yubikey to select it.

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
