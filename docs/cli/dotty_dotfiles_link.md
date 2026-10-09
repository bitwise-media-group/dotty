## dotty dotfiles link

Register, track, and apply the repository's home tree with mise.

### Synopsis

Link the repository's home/ tree over $HOME: whole files and directories
are linked folded, existing real directories are descended into, and stale
symlinks are replaced. A real file in the way is resolved per --on-conflict;
link defaults to fail so an unexpected file stops a re-link instead of being
moved. Legacy files that shadow the rendered configuration from outside any
link site (~/.gitconfig, ~/.zshrc and the other bare zsh startup files) are
backed up and removed; restore them with dotty dotfiles restore.

Deployment is handled by mise. Grok's configuration is copied as regular
files; other entries remain symlinks. link registers all repository entries
in the profile's mise config and tracks the shared and per-profile home
sources for history. Use dotty dotfiles add --changed to save live copy edits
back to their repository sources before re-linking. New files added with
dotty dotfiles add are captured into this repository's home/ tree.

```
dotty dotfiles link [flags]
```

### Examples

```
  dotty dotfiles link
  dotty dotfiles link --repo ~/Repos/dotfiles --on-conflict=backup
```

### Options

```
  -h, --help                 help for link
      --on-conflict string   existing-file resolution: backup, adopt, skip, or fail (default "fail")
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      dotfiles repository (default: found via $REPOS_DIR)
```

### SEE ALSO

* [dotty dotfiles](dotty_dotfiles.md)	 - Operate on the dotfiles repository dotty init generated.

