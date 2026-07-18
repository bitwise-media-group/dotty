## dotty git merge

Merge the current stack layer with its parent layer(s).

### Synopsis

Collapse parent layers into the current branch. A stacked child already
carries its parents' commits, so merging deletes the absorbed parent
branches (local and origin, honouring dotty.stack.cleanup) and removes them
from the stack — the current branch's history does not change.

By default the immediate parent is merged. --up=N absorbs the N layers below
the current one, and --all absorbs everything down to the bottom of the
stack. Asking for more parents than the stack has below the current layer is
an error.

The merge range must be in sync first: every absorbed parent's commits must
already be contained in the current branch. When they are not — a parent was
amended, or trunk moved under the bottom layer — merge offers to rebase and
re-sign the layers from the bottom of the stack up to the current one, then
proceeds. An open PR on an absorbed parent closes when its branch is
deleted; the current layer's PR carries the work.

```
dotty git merge [--all | --up=N] [flags]
```

### Examples

```
  dotty git merge
  dotty git merge --up=2
  dotty git merge --all
```

### Options

```
      --all      merge every layer between the current one and the bottom of the stack
  -h, --help     help for merge
      --up int   merge the current layer with this many parents (default 1)
  -y, --yes      rebase+resign an out-of-sync merge range without prompting
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.

