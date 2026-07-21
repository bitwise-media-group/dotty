## dotty git append

Create a child branch on the stack tip.

### Synopsis

Creates <branch> at the current stack tip, records it as a new
layer in the stack lineage, and pushes it to the push remote with upstream
tracking set.

```
dotty git append <branch> [flags]
```

### Examples

```
  dotty git append feat-2
```

### Options

```
  -h, --help   help for append
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.

