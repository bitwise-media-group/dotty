<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Credentials & the Keychain

[`dotty env`](../cli/dotty_env.md) is a small credential store backed by the
**macOS Keychain**: secrets are Keychain items, never plaintext files on disk,
and reach processes as environment variables only when you ask. Keys are grouped
into namespaces (`--namespace`, default `default`) so one project's secrets
don't collide with another's.

## Storing

```sh
dotty env add GITHUB_TOKEN                 # prompts for the value, hidden
dotty env add --in-file .env               # capture a whole .env file
dotty env --namespace=acme add API_KEY     # namespaced
```

[`dotty env add`](../cli/dotty_env_add.md) never echoes values and accepts an
existing `.env` file wholesale — a quick way to move a project's secrets off
disk (delete the file after).

## Reading

```sh
dotty env list                             # names only, values stay put
dotty env get GITHUB_TOKEN                 # one value to stdout
dotty env get dotty://acme/API_KEY         # URI form, namespace inline
```

[`dotty env get`](../cli/dotty_env_get.md) takes either a bare key (with
`--namespace`) or a `dotty://<namespace>/<key>` URI; `--no-newline` makes it
composable in command substitution.

## Injecting into processes

The sanctioned pattern — the secret exists in the child's environment for
exactly one command:

```sh
dotty env run -- gh api user
dotty env --namespace=acme run -- terraform plan
dotty env run --in-file .env.dotty -- npm test
```

[`dotty env run`](../cli/dotty_env_run.md) resolves the namespace's keys (or the
`dotty://` references in a template file) and executes the command with them
injected.

## Templates and round-tripping

[`dotty env use`](../cli/dotty_env_use.md) renders a template whose values are
`dotty://` references into a real `.env` file (`--out-file`) — for tools that
insist on reading one — and `--in-file` re-captures it. Keep the _template_
(references only, no secrets) in the repo; the global gitignore keeps
`.env.dotty` trackable while ignoring other `.env.*` files.

## Removing

```sh
dotty env remove API_KEY
dotty env --namespace=acme remove --all
```

## How this plays with agent hardening

The [hardened agent sandboxes](../reference/agent-sandboxing.md) deny reads of
`**/.env*` **and** block the macOS `security` CLI — a sandboxed agent can
neither read secret files nor rummage in the Keychain. `dotty env run` is the
intended escape hatch: _you_ run the command that needs the secret, outside the
agent, and the agent never sees the value.
