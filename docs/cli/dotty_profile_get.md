## dotty profile get

Show a profile's metadata and where it lives.

### Synopsis

Print a profile's metadata — name, description, creation date — alongside the
machine state around it: the profile directory, the dotfiles repository
directory it links to, whether it is the active profile, and how many entries
its Brewfile carries. Without a name dotty describes the active profile, or
the one the global --profile names.

--format=json prints profile.json verbatim instead, which for a profile dotty
init built also carries the wizard answers. That file knows nothing about this
machine, so the link target and active flag are text-mode only.

```
dotty profile get [<name>] [flags]
```

### Examples

```
  dotty profile get
  dotty profile get work
  dotty profile get work --format=json
```

### Options

```
      --format string   output format: text (metadata plus machine state) or json (profile.json verbatim) (default "text")
  -h, --help            help for get
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty profile](dotty_profile.md)	 - Manage system profiles that travel across machines.

