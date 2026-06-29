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
* [dotty git resign](dotty_git_resign.md)	 - Rebase and re-sign commits up to a commitish.

