## dotty tmux new

Start (or attach to) a repository's tmux dev session.

### Synopsis

Pick a repository and create-or-attach its tmux session: the editor on the
first window over a small shell split, one window per installed coding agent
(opencode, codex, claude), and a shell window. The session is named after the
repository, so rerunning the command attaches to the existing session.

Repositories are discovered up to four levels deep under $REPOS_DIR (default
~/Repos). A query narrows the picklist and auto-selects a single match; the
query "." searches from the current directory instead, which resolves to the
enclosing repository.

```
dotty tmux new [query] [flags]
```

### Examples

```
  dotty tmux new
  dotty tmux new dotty
  dotty tmux new .
```

### Options

```
  -h, --help   help for new
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty tmux](dotty_tmux.md)	 - Tmux sessions for agent-driven development.

