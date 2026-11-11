## dotty git worktree remove

Remove a worktree, its agent/* branch, and its tmux session.

### Synopsis

Tear a linked worktree down: kill the tmux session named after it, remove
the worktree from its repository, and delete its branch — but only an
agent/* branch, so a manually checked-out branch survives. A repository's
main checkout is never removed, and a worktree that is already gone exits
quietly.

Removal is forced past whatever git would normally refuse: populated
submodules, locks, and uncommitted changes. Work that would be lost —
uncommitted changes, or commits found on no remote and no other local branch
— is reported first and needs confirming: a prompt (defaulting to No) on a
terminal, --yes otherwise.

path falls back to the WorktreeRemove hook JSON on stdin
({"worktree_path": ...}), then the linked worktree enclosing the current
directory.

```
dotty git worktree remove [path] [flags]
```

### Examples

```
  dotty git worktree remove ~/.local/share/dotty/worktrees/dotty-fix-1
  dotty git worktree remove --yes        # the worktree you are in
  echo '{"worktree_path":"..."}' | dotty git worktree remove --yes   # hook form
```

### Options

```
  -f, --force[=true|false]   alias for --yes
  -h, --help[=true|false]    help for remove
  -y, --yes[=true|false]     remove without confirming uncommitted or unpushed work
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git worktree](dotty_git_worktree.md)	 - Manage agent git worktrees.

