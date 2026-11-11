## dotty git done

Return to trunk, prune merged branches everywhere, fast-forward.

### Synopsis

Finish a piece of work and reset to a clean, current trunk:

  1. Check out the trunk branch (main)
  2. Fetch all remotes with --prune
  3. Remove the worktrees of local branches already merged into trunk
     (prompting if a worktree has uncommitted changes)
  4. Delete every local branch already merged into trunk (upstream/main when
     present, else origin/main), dropping it from any recorded stack
  5. Delete every origin branch already merged into trunk
  6. Fast-forward the local trunk branch to the remote
  7. Push the trunk branch to origin (keeps a fork's main in step)

Step 3 is what collects agent worktrees whose removal hook never fired. A
worktree holding uncommitted changes is kept, along with its branch, unless
you confirm on a terminal; so is the worktree you are standing in.

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
  -h, --help[=true|false]   help for done
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.

