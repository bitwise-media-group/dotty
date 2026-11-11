## dotty tmux sidebar

Run the tmux session sidebar.

### Synopsis

Show every tmux session and its windows as a tree in a narrow pane, with each
coding agent's status: 󰂚 needs attention, ● is waiting for input. A session
row's badge counts its windows in the most urgent state.

This is the process the sidebar pane runs; tmux starts it (through the
sidebar.conf hooks or the toggle), and there is one for the whole server.
It moves itself into whichever window the client switches to, blocks on a
tmux wait-for channel between changes, and reloads the tree only when a hook
or an agent status change signals it.

Keys: j/k move, h/l collapse and expand, n/N jump to the next or previous
window needing you (attention first, then waiting), / filters, enter jumps to
the session or window, esc returns focus, q hides the sidebar. Clicking a row
jumps to it.

```
dotty tmux sidebar [flags]
```

### Examples

```
  dotty tmux sidebar toggle
  tmux split-window -h -f -l 32 'exec dotty tmux sidebar'
```

### Options

```
  -h, --help[=true|false]   help for sidebar
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty tmux](dotty_tmux.md)	 - Tmux sessions for agent-driven development.
* [dotty tmux sidebar toggle](dotty_tmux_sidebar_toggle.md)	 - Show, focus, or hide the tmux session sidebar.

