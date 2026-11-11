## dotty tmux sidebar toggle

Show, focus, or hide the tmux session sidebar.

### Synopsis

Cycle the session sidebar from the focused pane: hidden, it is shown and
focused; shown but unfocused, it is focused (moved into this window first if
it is elsewhere); focused, it is hidden. Hiding sets the @dotty_sidebar option
off, so switching windows no longer recreates it until the next toggle.

The tmux prefix binding (prefix s) and Neovim's session-sidebar mapping run
this.

```
dotty tmux sidebar toggle [flags]
```

### Examples

```
  dotty tmux sidebar toggle
  dotty tmux sidebar toggle --pane %3
```

### Options

```
  -h, --help[=true|false]   help for toggle
      --pane string         the focused pane (defaults to $TMUX_PANE, else the current client's pane)
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty tmux sidebar](dotty_tmux_sidebar.md)	 - Run the tmux session sidebar.

