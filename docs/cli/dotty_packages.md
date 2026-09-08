## dotty packages

Manage the profile's packages (mise tools and bootstrap packages).

### Synopsis

Maintain the profile's mise directory — config.toml, the component
fragments under conf.d/, and mise.lock — so a machine's packages stay
reproducible on and across systems. Two kinds of entry live there: tools
([tools], locked per platform in mise.lock, installed by mise itself) and
bootstrap packages ([bootstrap.packages], poured into the Homebrew prefix or
the OS package manager at their latest version). Commands operate on the
active profile, or on a specific profile's via the global --profile flag;
mise is installed into ~/.local/bin when the machine has none.

Package ids are full backend ids — aqua:sharkdp/bat, github:owner/repo,
npm:prettier, brew:git, brew-cask:ghostty — never bare registry names, which
resolve to whichever registry entry claims them.

### Examples

```
  dotty packages add aqua:sharkdp/bat brew-cask:raycast
  dotty packages sync
  dotty --profile=work packages status
  dotty pkg upgrade
```

### Options

```
  -h, --help   help for packages
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty packages add](dotty_packages_add.md)	 - Add packages to the profile and install them.
* [dotty packages edit](dotty_packages_edit.md)	 - Open the profile's config.toml in the default editor.
* [dotty packages import](dotty_packages_import.md)	 - Import installed Homebrew formulae, or a Brewfile, into the profile.
* [dotty packages lock](dotty_packages_lock.md)	 - Refresh mise.lock for every platform.
* [dotty packages remove](dotty_packages_remove.md)	 - Remove packages from the profile.
* [dotty packages status](dotty_packages_status.md)	 - List the profile's packages and whether they are installed.
* [dotty packages sync](dotty_packages_sync.md)	 - Make the machine match the profile's packages exactly.
* [dotty packages upgrade](dotty_packages_upgrade.md)	 - Upgrade every package the profile declares.

