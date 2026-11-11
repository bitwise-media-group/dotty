## dotty git worktree

Manage agent git worktrees.

### Synopsis

The agent worktree lifecycle: start creates or reuses a worktree on an
agent/<name> branch and prints its path; remove tears down the worktree, its
tmux session, and the agent/* branch. Both verbs also accept their agent hook
JSON on stdin (WorktreeCreate and WorktreeRemove), so they wire directly
into claude's hooks.

Worktrees live at the location the active profile configures (dotty init
--worktrees): one shared root outside every repository — the default
~/.local/share/dotty/worktrees — or a directory inside each repository.
Inside them, commit and tag signing is off (sandboxed agents cannot reach
the security key); re-sign afterwards with dotty git resign.

Agent hooks do not fire for every worktree (subagent worktrees never run
WorktreeRemove), so dotty git done also removes the worktrees of branches
it finds merged.

### Examples

```
  dotty git worktree start myrepo fix-tests
  dotty git worktree remove
```

### Options

```
  -h, --help[=true|false]   help for worktree
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty git](dotty_git.md)	 - Git helpers built on dotty's commit signing.
* [dotty git worktree remove](dotty_git_worktree_remove.md)	 - Remove a worktree, its agent/* branch, and its tmux session.
* [dotty git worktree start](dotty_git_worktree_start.md)	 - Create or reuse an agent/* worktree and print its path.

