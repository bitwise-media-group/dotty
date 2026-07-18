<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Signature-preserving stacks

Stacked work with **fork-only** remotes, **required signatures**, and a
**fast-forward merge strategy** — without rebase-on-land or Graphite.

## Model

Every stack PR targets **`upstream/main`** (or `origin/main` when there is no
upstream). Diffs are **cumulative** from trunk through that layer. The stack map
in each PR body is the navigation surface; GitHub base-branch chaining is not
used (parent branches cannot live on the org repo under fork-only).

```text
main
  \
   b1  ── PR → main   (layer 1 commits)
    \
     b2 ── PR → main   (layer 1+2 commits)
```

Landing is always a **fast-forward merge**: a ref move of `main` to the PR head,
so the signed commits land byte-for-byte. Merging a higher layer lands every
signed commit below it; lower PRs close when their tips become ancestors of
`main`. Stacks need no special support on the landing side — any fast-forward
mechanism works.

## Commands

```sh
dotty git start feat-api            # branch off trunk; new stack
# … commits (resign if they were agent-unsigned) …
dotty git append feat-ui            # child of tip
dotty git status                    # status of the stack vs trunk
dotty git propose                   # PRs for trunk..current
dotty git propose --all             # every layer
dotty git sync                      # cleanup merged; refresh maps; rebase+resign if diverged

# If you never ran start/append but already have a local chain
# main ← feat-a ← feat-b (3+ nodes), `dotty git status` discovers it and
# writes lineage into git config. A lone branch off main is not a stack.

dotty git up [n]                    # toward tip
dotty git down [n]                  # toward trunk
dotty git switch                    # fuzzy pick a layer (alias: stack)
dotty git browse                    # open upstream (else origin) in a browser
dotty git resign main               # re-sign after agent work or post-rebase
```

### `propose`

- Pushes to `origin` (fork).
- Opens/updates PRs against trunk on the base remote.
- Multi-commit layers: pick which commit supplies title + body.
- Body layout: stack map (dotty-owned markers) then `---` then commit body.

Default scope is **current layer down to trunk**. Mid-stack `propose` does not
open unfinished upper layers unless `--all`.

### `sync`

1. Fetch trunk and origin.
2. Layers whose tips are already on trunk: drop from lineage; by default
   **delete local + origin** branches (`git config dotty.stack.cleanup false` to
   keep them).
3. If any open layer **diverged** from trunk: prompt, then rebase the whole open
   stack bottom→tip and **re-sign** each layer. Conflicts:
   `dotty git sync --continue` or `--abort`.
4. Refresh stack maps on open PRs (merged rows checked off / removed).

Silent rebase without resign is forbidden — unsigned tips cannot land under
`required_signatures`.

## Land

Land stack PRs with a **true fast-forward**. GitHub's built-in merge methods
(merge commit, squash, rebase) all create new commits, which breaks the chain's
signatures — so the repository needs some fast-forward mechanism: a bot or
GitHub Action that moves `main` to the PR head, or a maintainer pushing the
fast-forward manually.

One implementation is the
[ff-merge GitHub Action](https://github.com/bitwise-media-group/ff-merge), where
you comment `/merge` on a ready PR or arm `/auto-merge` to land it when checks
pass.

Whatever the mechanism, prefer the lowest ready layer for an incremental land,
or a higher layer to land the whole chain at once.

## With agent worktrees

1. `dotty worktree start` / agent commits (unsigned).
2. `dotty git resign <trunk>` (or layer parent) before propose.
3. append/propose as above.

## Config

| Key                   | Default | Meaning                                      |
| --------------------- | ------- | -------------------------------------------- |
| `dotty.stack.cleanup` | true    | Delete local+origin when a layer is on trunk |
