## dotty git expand

Expand the current branch into a stack with one layer per commit.

### Synopsis

Turn the current branch's commits (trunk..HEAD) into a stack: every
commit gets its own layer branch, named from its subject, and the current
branch stays as the tip. Without --auto-squash no history is rewritten —
layer branches simply point at the existing commits.

With --auto-squash, each chore commit is first squashed into the commit
below it, so chores never form a layer of their own (a chore that is the
very first commit has nothing below it and keeps its own layer). Squashing
rewrites history from the first squash onward; rewritten commits are created
with plain git commits, so your commit signing configuration re-signs them.

The planned stack is always shown first and nothing changes until you
confirm. The branch is expanded in place — it is not rebased onto trunk; run
`dotty git propose` or `dotty git sync` afterwards as usual.

```
dotty git expand [--auto-squash] [flags]
```

### Examples

```
  dotty git expand
  dotty git expand --auto-squash
```

### Options

```
      --auto-squash   squash each chore commit into the commit below it before expanding
  -h, --help          help for expand
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.

