## dotty dotfiles

Operate on the dotfiles repository dotty init generated.

### Synopsis

Manage dotfiles through mise against the selected profile's mise config.
link registers the repository's home tree, applies symlinks by default and
regular copies for Grok, and tracks repository sources in mise history.
All mise dot commands and flags are available here. Use add --changed to
capture live copy edits back into the repository before applying it again.
restore with paths proxies rollback; --timestamp restores legacy dotty backups.

```
dotty dotfiles [flags]
```

### Examples

```
  dotty dotfiles status
  dotty dotfiles link --on-conflict=backup
  dotty dotfiles add --changed
  dotty dotfiles track ~/.zshrc
  dotty dotfiles rollback ~/.zshrc
```

### Options

```
  -h, --help          help for dotfiles
      --repo string   dotfiles repository (default: found via $REPOS_DIR)
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty dotfiles add](dotty_dotfiles_add.md)	 - Proxy mise dot add against the selected profile.
* [dotty dotfiles apply](dotty_dotfiles_apply.md)	 - Proxy mise dot apply against the selected profile.
* [dotty dotfiles capture](dotty_dotfiles_capture.md)	 - Proxy mise dot capture against the selected profile.
* [dotty dotfiles conflicts](dotty_dotfiles_conflicts.md)	 - Proxy mise dot conflicts against the selected profile.
* [dotty dotfiles diff](dotty_dotfiles_diff.md)	 - Proxy mise dot diff against the selected profile.
* [dotty dotfiles edit](dotty_dotfiles_edit.md)	 - Proxy mise dot edit against the selected profile.
* [dotty dotfiles exclude](dotty_dotfiles_exclude.md)	 - Proxy mise dot exclude against the selected profile.
* [dotty dotfiles help](dotty_dotfiles_help.md)	 - Proxy mise dot help against the selected profile.
* [dotty dotfiles history](dotty_dotfiles_history.md)	 - Proxy mise dot history against the selected profile.
* [dotty dotfiles include](dotty_dotfiles_include.md)	 - Proxy mise dot include against the selected profile.
* [dotty dotfiles link](dotty_dotfiles_link.md)	 - Register, track, and apply the repository's home tree with mise.
* [dotty dotfiles origin](dotty_dotfiles_origin.md)	 - Proxy mise dot origin against the selected profile.
* [dotty dotfiles paths](dotty_dotfiles_paths.md)	 - Proxy mise dot paths against the selected profile.
* [dotty dotfiles pull](dotty_dotfiles_pull.md)	 - Proxy mise dot pull against the selected profile.
* [dotty dotfiles recover](dotty_dotfiles_recover.md)	 - Proxy mise dot recover against the selected profile.
* [dotty dotfiles restore](dotty_dotfiles_restore.md)	 - Restore mise history or an existing dotty backup set.
* [dotty dotfiles rollback](dotty_dotfiles_rollback.md)	 - Proxy mise dot rollback against the selected profile.
* [dotty dotfiles save](dotty_dotfiles_save.md)	 - Proxy mise dot save against the selected profile.
* [dotty dotfiles status](dotty_dotfiles_status.md)	 - Show mise's dotfile deployment and tracking status.
* [dotty dotfiles sync](dotty_dotfiles_sync.md)	 - Proxy mise dot sync against the selected profile.
* [dotty dotfiles track](dotty_dotfiles_track.md)	 - Proxy mise dot track against the selected profile.
* [dotty dotfiles unapply](dotty_dotfiles_unapply.md)	 - Proxy mise dot unapply against the selected profile.
* [dotty dotfiles undo](dotty_dotfiles_undo.md)	 - Proxy mise dot undo against the selected profile.
* [dotty dotfiles untrack](dotty_dotfiles_untrack.md)	 - Proxy mise dot untrack against the selected profile.
* [dotty dotfiles watch](dotty_dotfiles_watch.md)	 - Proxy mise dot watch against the selected profile.

