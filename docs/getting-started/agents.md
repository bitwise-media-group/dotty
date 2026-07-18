<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Coding agents & hardening

dotty scaffolds configuration for four terminal coding agents — **Claude
Code**, **Codex**, **OpenCode**, and **Grok** — plus the Antigravity cask,
and can confine all of them behind one consistent security policy. Pick
agents in the wizard or with `--agents=claude-code,codex,…`; each
selection adds the agent's Brewfile entry and renders its config into your
repo under `home/.config/<agent>/`.

## What `--harden` does

Hardening mirrors one policy into every agent's native config:

- **An OS-level sandbox** (macOS Seatbelt) restricting writes to your
  repos, worktrees, and tool caches.
- **Credential-read denies** — `~/.ssh`, `~/.aws`, `~/.azure`,
  `~/.config/gcloud`, `~/.gnupg`, and any `.env` file are unreadable, and
  the macOS Keychain is unreachable.
- **A network allowlist** (Claude Code) limited to code hosts and package
  registries.
- **Ask-first approvals** — only read-only inspection commands and safe
  git operations are pre-approved.

!!! warning "Hardening is opt-in at init"

    Without `--harden`, agents get theme, hooks, and marketplace config
    only — a stock install. The full per-agent policy, including how
    Codex, Grok, and OpenCode compensate for sandbox features they lack,
    is documented in [Agent sandboxing](../reference/agent-sandboxing.md).

## One memory doc for every agent

All selected agents share a single instructions file: it renders once (as
`CLAUDE.md` when Claude Code is selected) and the other agents'
`AGENTS.md` are hard links to it, so the rules can never drift apart. It
teaches the habits the sandbox assumes: temp files under `$TMPDIR`,
Conventional Commit messages, and the `commit.sh` hand-off for signed
commits (see [Agent worktrees & re-signing](../guides/worktrees.md)).

## Agent sessions in tmux

With the tmux addon, [`dotty tmux new`](../cli/dotty_tmux_new.md) opens a
development session for a repo with a window per installed agent — or
launch one ad hoc with ++ctrl+a++ then ++shift+c++ (Claude Code),
++shift+g++ (Grok), ++shift+o++ (OpenCode), or ++shift+x++ (Codex). Every
agent reports into the status bar via lifecycle hooks: a peach ● when it's
finished and waiting, a red 🔔 when it needs an approval. Details in the
[tmux reference](../reference/tmux.md).

## Worktrees, in one paragraph

Because hardened agents can't reach your signing key, dotty gives them git
worktrees (on `agent/*` branches) where signing is disabled, wired
automatically into Claude Code's worktree hooks; when the work is done you
re-sign the branch with
[`dotty git resign`](../cli/dotty_git_resign.md). The full flow — including
where worktrees live and the `commit.sh` convention — is in
[Agent worktrees & re-signing](../guides/worktrees.md).

[Next: Daily workflow →](daily-workflow.md)
