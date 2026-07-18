<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

<!-- Source of truth: internal/scaffold/template/home/.config/{claude,codex,grok,opencode}/
     — update this page when those templates change. -->

# Agent sandboxing

When you answer yes to "Harden the coding agents?" (or pass `--harden` to
[`dotty init`](../cli/dotty_init.md)), dotty renders every selected agent's
configuration with a consistent confinement policy. The threat model is simple:
an agent that goes wrong — or gets prompt-injected — should not be able to
**read your credentials** (`~/.ssh`, cloud CLI configs, `.env` files, the macOS
keychain), **write outside your repos**, or **talk to arbitrary hosts** without
you approving it first.

Claude Code's sandbox is the reference policy; Codex, Grok, and OpenCode mirror
it with their own primitives, filling the gaps with hooks where their sandboxes
can't express a rule. Without `--harden`, agents get only the non-security parts
(theme, tmux status hooks, marketplace, shared memory doc).

!!! note "The rendered files are yours"

    Everything below describes the templates as dotty ships them **at init
    time**. The rendered configs land in *your* dotfiles repo
    (`home/.config/<agent>/`) and are yours to edit afterwards — dotty never
    rewrites them behind your back.

## The shared baseline

|                  | Claude Code                     | Codex                           | Grok                                 | OpenCode                      |
| ---------------- | ------------------------------- | ------------------------------- | ------------------------------------ | ----------------------------- |
| Config file      | `settings.json`                 | `config.toml`                   | `config.toml` + `sandbox.toml`       | `opencode.jsonc`              |
| OS sandbox       | Seatbelt (full: FS + network)   | Seatbelt (`workspace-write`)    | Seatbelt (custom `dotfiles` profile) | — (permission layer only)     |
| Network policy   | per-domain allowlist            | on/off only (on)                | unrestricted                         | webfetch: ask                 |
| Credential reads | sandbox `denyRead`              | PreToolUse hook                 | kernel `deny` + hook                 | permission denies             |
| Keychain block   | no `SecurityServer` mach lookup | hook denies `security` CLI      | `Bash(security *)` deny + hook       | `security *` bash deny        |
| Approval default | ask unless allow-listed         | `approval_policy = "untrusted"` | prompt + explicit allows             | `"*": "ask"`                  |
| tmux status      | hooks in `settings.json`        | `[[hooks.*]]` in `config.toml`  | `hooks/tmux-status.json`             | `plugin/agent-tmux-status.js` |

All four share the same Bash allowlist (read-only inspection commands, read-only
`brew` and `git`, plus `git add`/`restore`/`checkout`/`switch`/ `commit`) and
deny the same credential set: `~/.aws`, `~/.azure`, `~/.config/gcloud`,
`~/.ssh`, `~/.gnupg`, and `**/.env*`.

## Claude Code

`~/.config/claude/settings.json` carries the canonical policy, enforced by
Claude Code's OS-level (Seatbelt) sandbox:

- **Sandbox**: `enabled: true`, with `autoAllowBashIfSandboxed: false` (being
  sandboxed does not skip approval) and `allowUnsandboxedCommands: false`
  (nothing runs outside the sandbox).
- **Filesystem**: writes allowed only under `~/Repos`, the agent-worktrees root
  (when configured as an absolute path), and tool caches (`~/.cache`,
  `~/.local/share/go`, `~/.local/bin`, `~/.npm`, `~/.config/helm`,
  `~/.kubescape`, `~/.sigstore`, `/tmp`). Reads are allowed under `~/Repos`,
  `~/.config`, `~/.cache`, `~/.local/{bin,runtime,share}`, `~/.npm`,
  `/opt/homebrew`, and `/tmp` — and **denied** (read and write) for the
  credential set.
- **Keychain**: the sandbox's mach-lookup allowlist includes only trust
  evaluation and DNS/Open Directory services — notably _not_
  `com.apple.SecurityServer`, which is what blocks macOS keychain access.
- **Permissions**: read-only tools (`Read`, `Glob`, `Grep`, `WebSearch`) and the
  shared Bash allowlist are pre-approved; edits are pre-approved only under
  `/tmp` and the worktrees root; everything else asks (`defaultMode: auto`).

??? note "Full network allowlist"

    Egress is limited to these domains, grouped by purpose:

    - **Code hosting**: `github.com`, `api.github.com`,
      `objects.githubusercontent.com`, `raw.githubusercontent.com`,
      `codeload.github.com`, `ghcr.io`, `codeberg.org`, `code.forgejo.org`
    - **Package registries**: `pypi.org`, `files.pythonhosted.org`,
      `registry.npmjs.org`, `nodejs.org`, `formulae.brew.sh`,
      `proxy.golang.org`, `sum.golang.org`, `index.golang.org`,
      `vuln.go.dev`, `registry.terraform.io`
    - **Container registries**: `docker.io`, `registry-1.docker.io`,
      `auth.docker.io`, `index.docker.io`,
      `production.cloudflare.docker.com`, `quay.io`, `*.quay.io`,
      `*.pkg.dev`, `public.ecr.aws`
    - **Kubernetes / supply chain**: `registry.k8s.io`, `registry.istio.io`,
      `istio-release.storage.googleapis.com`,
      `prometheus-community.github.io`, `grype.anchore.io`,
      `toolbox-data.anchore.io`, `*.sigstore.dev`
    - **AWS**: `*.amazonaws.com`,
      `*.s3.dualstack.eu-west-1.amazonaws.com`,
      `*.s3.dualstack.eu-west-2.amazonaws.com`
    - **Fonts**: `fonts.googleapis.com`, `fonts.gstatic.com`
    - **Local**: `localhost`, `127.0.0.1`, `::1`, Unix sockets under `/tmp`
      and `/private/tmp`, and local port binding for dev servers.

## Codex

`~/.config/codex/config.toml` recovers the same guarantees with Codex's
primitives:

- **Sandbox**: `sandbox_mode = "workspace-write"` — the same Seatbelt mechanism,
  but it restricts _writes only_: the working directory plus `writable_roots`
  (repos dir, worktrees root when absolute, `~/.cache`, `~/.local/share/go`,
  `~/.npm`), with `/tmp` and `$TMPDIR` kept writable.
- **Approvals**: `approval_policy = "untrusted"` — prompt for anything the
  execution policy hasn't explicitly trusted. `rules/default.rules` is a
  `prefix_rule` allowlist mirroring the shared Bash set, with the macOS
  `security` tool marked `forbidden`.
- **Network**: Codex can only toggle network access wholesale, so
  `network_access = true` keeps installs working. This is broader than Claude
  Code's per-domain allowlist — the credential guard below still applies
  regardless.
- **Credential guard**: because a write-only sandbox cannot block _reads_, a
  `PreToolUse` hook (`hooks/pre-tool-use-policy`, matcher `^Bash$`) inspects
  every shell command and denies any that reference a credential path or the
  `security` keychain CLI, returning a reason the model can act on.

## Grok

`~/.config/grok/config.toml` selects a custom sandbox profile and mirrors the
permission lists:

- **Sandbox**: `[sandbox] profile = "dotfiles"`, defined in `sandbox.toml`. It
  extends Grok's `workspace` profile with the same writable roots as the other
  agents and **kernel-enforced denies** of the credential paths (Seatbelt blocks
  both reads and writes there). Paths in `sandbox.toml` must be absolute — Grok
  does not expand `~`.
- **Permissions**: the shared allow set, with denies written as `**/…` globs
  (Grok treats a leading `~/` as literal text in Read/Edit rules) plus
  `Bash(security *)` for the keychain.
- **Credential guard**: the same `pre-tool-use-policy` script as Codex, wired
  via `hooks/pre-tool-use-policy.json`, catches home-anchored paths the glob
  rules can't.
- **Limits**: Grok's workspace-class profiles have no per-domain network
  allowlist, so child-process network is unrestricted — the hook and kernel
  denies are the compensating controls.
- **Extras**: `disable_codebase_upload = true`, telemetry and feedback off, and
  `[compat.claude]` keeps reading Claude Code agents/sessions while disabling
  its skills/rules/MCPs/hooks so nothing fires twice.

## OpenCode

`~/.config/opencode/opencode.jsonc` has no OS sandbox; the whole policy is one
permission matrix with `"*": "ask"` as the default:

- **read / list / glob**: allowed everywhere except the credential set.
- **edit**: allowed in the worktrees root and `/tmp`; asks in `~/Repos` and tool
  caches; denies `~/.config`, `~/.local`, `/opt/homebrew`, and the credential
  set.
- **external_directory**: default deny, with an allowlist matching Claude Code's
  read scope (`~/Repos`, `~/.config`, `~/.cache`, `~/.local/{runtime,share}`,
  `~/.npm`, `/opt/homebrew`, `/tmp`).
- **bash**: the shared allowlist, plus explicit argument-position denies
  (`security *`, `* ~/.ssh*`, `* $HOME/.aws*`, `* */.env*`, …) since there is no
  OS layer underneath to catch what slips through.
- **web**: `websearch: allow`, `webfetch: ask`, and `doom_loop: deny`.

## Shared guardrails

Beyond confinement, every agent gets the same working agreements:

- **One memory doc.** The shared agent instructions render once at the primary
  agent's path (`CLAUDE.md` when Claude Code is selected) and the other agents'
  `AGENTS.md` files are hard links to it, so the rules cannot drift. It covers
  using `$TMPDIR` for temporary files, Conventional Commits, and the `commit.sh`
  hand-off below.
- **The `commit.sh` signing hand-off.** Hardened agents cannot reach the YubiKey
  to sign commits, so instead of signing they write a `commit.sh` script
  containing the exact commit (or re-signing) commands for you to run outside
  the sandbox. `commit.sh` is in the global gitignore. See
  [Agent worktrees & re-signing](../guides/worktrees.md).
- **tmux status.** All four agents report their state (waiting / needs
  attention) into the tmux status bar via `dotty tmux set-status` — see
  [tmux](tmux.md).

## Customising

Tighten or loosen any of this by editing the rendered files in your repo and
re-linking (`dotty dotfiles link`). Common tweaks: adding a domain to Claude
Code's `network.allowedDomains`, trusting another command prefix in Codex's
`rules/default.rules`, or promoting an OpenCode `ask` to `allow`. Keep the
credential denies — they are the point.
