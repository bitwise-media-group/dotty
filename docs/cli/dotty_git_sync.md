## dotty git sync

Fetch trunk, clean merged layers, refresh PR maps, rebase+resign if diverged.

### Synopsis

Synchronise the current stack with trunk:

  1. Fetch upstream/origin
  2. Drop layers already on trunk — including one that landed by
     fast-forward, which only its PR state distinguishes from an empty layer
     sitting at the trunk tip (default: delete local + origin branches)
  3. If any open layer diverged from trunk — or no longer contains the layer
     below it, because new commits landed mid-stack — prompt to rebase the
     open stack and re-sign each rewritten layer (use --continue / --abort
     around conflicts)
  4. Force-with-lease push the rewritten branches and return to the branch
     the sync started on
  5. Rewrite any open PR whose title or body is stale — a stacked layer owns
     both, so they come from the commit that layer nominated; amend it to
     change a description, rather than editing the PR on GitHub

Config: git config dotty.stack.cleanup false  # keep merged branches

```
dotty git sync [--continue | --abort] [flags]
```

### Examples

```
  dotty git sync
  dotty git sync --continue
  dotty git sync --yes
```

### Options

```
      --abort      abort an in-progress stack rebase
      --continue   resume after resolving rebase conflicts
  -h, --help       help for sync
  -y, --yes        rebase+resign without prompting when diverged
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.

