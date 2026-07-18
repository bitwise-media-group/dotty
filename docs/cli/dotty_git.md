## dotty git

Git helpers built on dotty's commit signing.

### Synopsis

Helpers that drive git through dotty's hardware-backed signing. Set signing
up first with `dotty signing-key sign --print-git-config`.

### Examples

```
  dotty git resign HEAD~3
  dotty git resign --root --reset-author
```

### Options

```
  -h, --help   help for git
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty git append](dotty_git_append.md)	 - Create a child branch on the stack tip.
* [dotty git browse](dotty_git_browse.md)	 - Open the upstream (else origin) repository page in a browser.
* [dotty git done](dotty_git_done.md)	 - Return to trunk, prune merged branches everywhere, fast-forward.
* [dotty git down](dotty_git_down.md)	 - Move toward the trunk of the current stack.
* [dotty git expand](dotty_git_expand.md)	 - Expand the current branch into a stack with one layer per commit.
* [dotty git merge](dotty_git_merge.md)	 - Merge the current stack layer with its parent layer(s).
* [dotty git propose](dotty_git_propose.md)	 - Open or update trunk-based PRs for the stack.
* [dotty git resign](dotty_git_resign.md)	 - Rebase and re-sign commits up to a commitish.
* [dotty git stack](dotty_git_stack.md)	 - Show the current stack versus trunk.
* [dotty git start](dotty_git_start.md)	 - Create a branch from trunk and start a new stack.
* [dotty git switch](dotty_git_switch.md)	 - Pick a stack layer and check it out.
* [dotty git sync](dotty_git_sync.md)	 - Fetch trunk, clean merged layers, refresh PR maps, rebase+resign if diverged.
* [dotty git up](dotty_git_up.md)	 - Move toward the tip of the current stack.

