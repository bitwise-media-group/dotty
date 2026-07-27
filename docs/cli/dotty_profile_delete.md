## dotty profile delete

Delete a profile and the directory behind it.

### Synopsis

Delete the named profile, or the active one when no name is given. A
profile's content lives in the dotfiles repository and the config directory
only links to it, so deleting removes both the
$XDG_CONFIG_HOME/dotty/<name> entry and the profiles/<name> directory it
resolves to — unlinking alone would leave the next dotty dotfiles link to put
it straight back. Commit that removal to finish the job.

The last profile cannot be deleted, and deleting the active one means naming
its replacement first: --activate does that, and without it dotty asks. dotty
confirms before removing anything on a terminal; --yes skips the prompt, which
is also what a non-interactive run does.

```
dotty profile delete [<name>] [flags]
```

### Examples

```
  dotty profile delete work
  dotty profile delete --activate=personal
  dotty profile rm work --yes
```

### Options

```
      --activate string   profile to activate in place of the deleted one
  -h, --help              help for delete
      --yes               skip the confirmation prompt
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty profile](dotty_profile.md)	 - Manage system profiles that travel across machines.

