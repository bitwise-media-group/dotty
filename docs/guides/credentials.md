<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Secrets with fnox

Secrets are managed by [fnox](https://fnox.jdx.dev): values are age-encrypted in
TOML — a global `~/.config/fnox/config.toml` that travels with your dotfiles,
and a `fnox.toml` per project — and reach processes as environment variables
only when you ask. dotty installs fnox with its core packages, activates its
shell hook, and sets up the encryption so you never manage a key by hand.

## Setup

```sh
dotty packages sync      # installs fnox and age
dotty env migrate        # sets up the providers (and migrates anything old)
```

[`dotty env migrate`](../cli/dotty_env_migrate.md) is idempotent. Its first run
generates a software age identity, parks it in the **macOS Keychain** through
fnox's keychain provider (decrypting never prompts for a touch), and writes the
`dotty-age` provider encrypting to that identity **plus every security key
enrolled for your private dotfiles** — a backup YubiKey can decrypt everything
on a machine that lacks the software key. The config holds only ciphertext and
public keys, so it is safe to commit; `dotty init` also runs this step when fnox
is already installed.

The first `fnox get` triggers the Keychain allow dialog once. Choose **Always
Allow**.

## Storing

```sh
fnox set GITHUB_TOKEN                       # prompts for the value, hidden
printf '%s' "$TOKEN" | fnox set GITHUB_TOKEN # or from stdin
fnox set -P acme API_KEY                    # in the "acme" profile
fnox set --default=8080 PORT                # a plain, non-secret default
```

Inside a project, `fnox set` writes to its `fnox.toml` (create it with
`fnox init`); add `-g` for the global config. Profiles (`-P`) replace the old
namespaces: one project's secrets don't collide with another's, and
`FNOX_PROFILE` selects one for a whole session. `fnox.local.toml` holds
per-machine overrides and is globally ignored.

## Reading

```sh
fnox list                                   # names only, values stay put
fnox get GITHUB_TOKEN                       # one value to stdout
fnox -P acme get API_KEY
```

## Injecting into processes

The sanctioned pattern — the secret exists in the child's environment for
exactly one command:

```sh
fnox exec -- gh api user
fnox -P acme exec -- terraform plan
```

dotty's config sets `env = "exec"`, so secrets **only** enter `fnox exec`
subprocesses. The shell hook (`fnox activate zsh`) is loaded, but with this
setting it never exports secrets into your interactive shell — which is where
coding agents run. For a tool that insists on a `.env` file:

```sh
fnox export --all -o .env      # then delete it when done; .env.* is ignored
```

## Migrating from `dotty env`

The old verbs are gone. `dotty env migrate` carries the old store over:

| Was                    | Now                         |
| ---------------------- | --------------------------- |
| `dotty env add KEY`    | `fnox set KEY`              |
| `dotty env get KEY`    | `fnox get KEY`              |
| `dotty env list`       | `fnox list`                 |
| `dotty env remove KEY` | `fnox remove KEY`           |
| `dotty env run -- cmd` | `fnox exec -- cmd`          |
| `dotty env use`        | `fnox export --all -o .env` |
| `--namespace=ns`       | `-P ns`                     |
| `.env.dotty` template  | `fnox.toml` beside it       |

```sh
dotty env migrate --dry-run             # what would move
dotty env migrate                       # every namespace, plus ./.env.dotty if present
dotty env migrate --namespace acme --purge   # one namespace, then delete its keychain item
dotty env migrate --skip-keychain svc/.env.dotty
```

Every keychain namespace becomes a profile in the global config (`default` lands
in the top-level secrets). Each template becomes the `fnox.toml` beside it:
plain `KEY=value` lines as defaults, `{{ dotty://ns/KEY }}` references as
encrypted values. Re-running is safe — keys fnox already has are skipped unless
`--force` — and problems are reported per line without stopping the run:

- an unterminated quote or malformed reference;
- a value mixing a reference with text (`postgres://{{ … }}/db`): store the
  secret on its own and use fnox's `default = "postgres://${HOST}/db"`
  interpolation;
- fnox trims surrounding whitespace from values.

Nothing old is deleted unless you pass `--purge` (confirmed, and only for a
namespace whose every credential migrated). Delete a template yourself once
`fnox exec` works in its directory, then run `fnox check --all`.

## Recovery with a backup key

The software identity lives in one machine's Keychain. On a new machine with a
YubiKey that was enrolled for your private dotfiles:

```sh
FNOX_AGE_KEY_FILE=~/Repos/dotfiles.private/profiles/<profile>/age/identity-<serial>.txt dotty env migrate
fnox reencrypt
```

The first command decrypts with the key (one touch) while bootstrapping a fresh
software identity for this machine; `fnox reencrypt` adds it to every value.
Enrolled a key after setting fnox up? Run `dotty env migrate` again — it only
adds providers when they are missing, so append the recipient to the `dotty-age`
provider in `config.toml` and run `fnox reencrypt`.

## Private dotfiles stay on age

fnox holds values, not files: it cannot encrypt a file in place at a stable
path, carry mode bits, or track which deployed file is stale.
[Private dotfiles](private.md) — `~/.ssh/config.d`, the private git config —
therefore stay on the `age` CLI and your security keys.

## How this plays with agent hardening

The [hardened agent sandboxes](../reference/agent-sandboxing.md) deny reads of
`**/.env*`, block the macOS `security` CLI, **and deny `fnox`** — a sandboxed
agent can neither read secret files, rummage in the Keychain, nor ask fnox for a
value. `fnox exec` is the intended escape hatch: _you_ run the command that
needs the secret, outside the agent, and the agent never sees the value.
