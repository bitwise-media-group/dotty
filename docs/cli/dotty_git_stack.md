## dotty git stack

Show the current stack versus trunk.

### Synopsis

Print the current branch's stack: each layer, its relation to trunk
(ff, merged, diverged, identical), and any linked PR numbers.

If this branch is not yet recorded in git config but local branches form an
obvious linear lineage of at least three nodes (trunk plus two or more feature
branches), the lineage is detected and saved automatically. A single branch off
trunk is not treated as a stack.

This is not git status — it is the status of the stacked branch chain managed
by start / append / propose / sync.

```
dotty git stack [flags]
```

### Examples

```
  dotty git stack
```

### Options

```
  -h, --help   help for stack
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.

