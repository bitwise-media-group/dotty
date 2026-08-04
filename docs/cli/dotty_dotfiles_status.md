## dotty dotfiles status

Report the link state of the dotfiles tree without changing it.

### Synopsis

Walk the repository's home/ tree against $HOME and report each entry: ok
(linked correctly), missing (link would be created), stale (a symlink
pointing elsewhere), or conflict (a real file in the way). Copy-deployed
directories (grok's hooks) report the same states for their mirrored files:
stale means the mirror is out of sync or still the retired symlink.

```
dotty dotfiles status [flags]
```

### Examples

```
  dotty dotfiles status
```

### Options

```
  -h, --help   help for status
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      dotfiles repository (default: found via $REPOS_DIR)
```

### SEE ALSO

* [dotty dotfiles](dotty_dotfiles.md)	 - Operate on the dotfiles repository dotty init generated.

