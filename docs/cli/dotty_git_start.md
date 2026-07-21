## dotty git start

Create a branch from trunk and start a new stack.

### Synopsis

Creates <branch> from the trunk (upstream/main when present, else
origin/main), records it as the first layer of a new stack, and pushes it
to the push remote with upstream tracking set.

```
dotty git start <branch> [flags]
```

### Examples

```
  dotty git start feat-1
```

### Options

```
  -h, --help   help for start
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.

