## dotty profile

Manage system profiles that travel across machines.

### Synopsis

Profiles are per-machine configuration sets — a Brewfile today; prompt and
terminal themes later — stored under $XDG_CONFIG_HOME/dotty/<name> so a public
dotfiles repository can carry them. One profile is active at a time, named by
the active-profile symlink.

### Examples

```
  dotty profile new --name=work --description="work laptop"
  dotty profile list
  dotty profile activate
  dotty profile activate --name=personal
```

### Options

```
  -h, --help   help for profile
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty profile activate](dotty_profile_activate.md)	 - Activate an existing profile.
* [dotty profile delete](dotty_profile_delete.md)	 - Delete a profile and the directory behind it.
* [dotty profile get](dotty_profile_get.md)	 - Show a profile's metadata and where it lives.
* [dotty profile list](dotty_profile_list.md)	 - List the profiles available on this machine.
* [dotty profile new](dotty_profile_new.md)	 - Create a system-level profile.

