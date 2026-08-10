## dotty brewfile

Manage the profile's Brewfile for reproducible brews.

### Synopsis

Maintain a homebrew bundle Brewfile so a machine's brews stay reproducible
on and across systems. Commands operate on the active profile's Brewfile, or
on a specific profile's via the global --profile flag.

### Examples

```
  dotty brewfile add ripgrep
  dotty brewfile add --cask ghostty
  dotty --profile=work brewfile sync
  dotty brew upgrade
```

### Options

```
  -h, --help   help for brewfile
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty brewfile add](dotty_brewfile_add.md)	 - Add brews to the Brewfile and install them.
* [dotty brewfile dump](dotty_brewfile_dump.md)	 - Snapshot the installed brews into the Brewfile.
* [dotty brewfile edit](dotty_brewfile_edit.md)	 - Open the Brewfile in the default editor.
* [dotty brewfile remove](dotty_brewfile_remove.md)	 - Remove entries from the Brewfile.
* [dotty brewfile sync](dotty_brewfile_sync.md)	 - Make the machine match the Brewfile exactly.
* [dotty brewfile upgrade](dotty_brewfile_upgrade.md)	 - Upgrade everything in the Brewfile.

