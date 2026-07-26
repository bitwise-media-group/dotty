## dotty git propose

Open or update trunk-based PRs for the stack.

### Synopsis

Push stack branches and open pull requests against upstream/main
(or origin/main). Default: layers from the trunk through the current branch.
With --all, propose every layer in the stack.

A branch without stack lineage works too: propose adopts it first — as a
discovered chain when the local branch topology makes one obvious, otherwise
as a new single-layer stack.

Before anything is pushed the stack is reconciled with the remote: any layer
whose PR has already landed is dropped, its local and origin branches deleted
— `dotty git done`'s cleanup, scoped to this stack and without leaving
your branch. Keep the branches with `git config set dotty.stack.cleanup false`.

Every layer that remains must then be up to date with trunk
(fast-forwardable) and with the layers below it; if the stack has diverged or
a lower layer gained commits, you are prompted to rebase + resign, as
`dotty git sync` does.

Each PR body includes a stack map with links. For multi-commit layers you pick
which commit supplies the title and description. A stacked layer owns its PR's
title and body: both are rewritten from that commit on every run, so change a
description by amending the commit, not by editing the PR on GitHub. A layer
whose PR dotty does not know about — opened by hand, or before the branch
joined a stack — adopts the open PR for that branch instead of colliding with
it, and that PR is dotty's from then on.

With --auto-merge=<merge|rebase|squash>, each proposed PR is flagged to merge
automatically with that method once its requirements pass. If the repository
has auto-merge switched off, or disallows the chosen method, propose warns and
continues. --auto-merge=comment instead posts a `/auto-merge` comment
on each PR, for repositories where a merge bot watches for it.

With --draft, each PR opens as a draft. Drafts cannot merge, so --draft and
--auto-merge are mutually exclusive and a configured auto-merge default is
ignored for the run. --draft applies when a PR is opened; an existing PR keeps
its draft state. Proposing again without --draft takes any draft PR out of
draft first, then applies auto-merge to it as usual.

With --browse, each proposed PR opens in your browser afterwards; with --copy,
the PR URLs (one per line) land on your clipboard. Make any of these the
default via git configuration: `git config set dotty.propose.browse true`
(and dotty.propose.copy, dotty.propose.auto-merge, dotty.propose.draft).

```
dotty git propose [--all] [--auto-merge=merge|rebase|squash|comment] [--browse] [--copy] [--draft] [flags]
```

### Examples

```
  dotty git propose
  dotty git propose --all
  dotty git propose --auto-merge=rebase
  dotty git propose --auto-merge=comment
  dotty git propose --draft
  dotty git propose --browse --copy
```

### Options

```
      --all               propose every layer in the stack, not only through the current branch
      --auto-merge mode   auto-merge each proposed pull request with the given method (merge, rebase, or squash); mode comment posts a /auto-merge comment for a merge bot instead
      --browse            open each proposed pull request in the browser
      --copy              copy the proposed pull request URL(s) to the clipboard
      --draft             open each pull request as a draft; proposing again without it readies them
  -h, --help              help for propose
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.

