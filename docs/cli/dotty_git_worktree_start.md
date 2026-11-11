## dotty git worktree start

Create or reuse an agent/* worktree and print its path.

### Synopsis

Create (or reuse) a worktree for repo on an agent/<repo>-<suffix> branch at
the configured worktree location, and print its path — the path is the
command's result, so hooks and scripts capture stdout.

repo falls back to $CLAUDE_PROJECT_DIR, then the enclosing repository.
suffix falls back to the WorktreeCreate hook JSON on stdin ({"name": ...}),
then a UTC timestamp.

```
dotty git worktree start [repo] [suffix] [flags]
```

### Examples

```
  dotty git worktree start
  dotty git worktree start ~/Repos/dotty fix-linker
  echo '{"name":"fix-1"}' | dotty git worktree start   # hook form
```

### Options

```
  -h, --help[=true|false]   help for start
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git worktree](dotty_git_worktree.md)	 - Manage agent git worktrees.

