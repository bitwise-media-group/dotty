## dotty git done

Return to trunk, prune merged branches everywhere, fast-forward.

### Synopsis

Finish a piece of work and reset to a clean, current trunk:

  1. Check out the trunk branch (main)
  2. Fetch all remotes with --prune
  3. Delete every local branch already merged into trunk (upstream/main when
     present, else origin/main), dropping it from any recorded stack
  4. Delete every origin branch already merged into trunk
  5. Fast-forward the local trunk branch to the remote

Any remaining stack that has diverged from trunk is reported; check it out
and run `dotty git sync` to rebase and re-sign it.

```
dotty git done [flags]
```

### Examples

```
  dotty git done
```

### Options

```
  -h, --help   help for done
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.

