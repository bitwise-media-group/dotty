## dotty tmux

Tmux sessions for agent-driven development.

### Synopsis

Start and attach tmux dev sessions laid out for coding agents: an editor
window with a small shell split, one window per installed agent (opencode,
grok, codex, claude), and a shell window, all named after the repository.

### Examples

```
  dotty tmux new
  dotty tmux new dotty
```

### Options

```
  -h, --help   help for tmux
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty tmux new](dotty_tmux_new.md)	 - Start (or attach to) a repository's tmux dev session.

