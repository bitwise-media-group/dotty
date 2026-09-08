## dotty packages remove

Remove packages from the profile.

### Synopsis

Remove one or more packages from the profile's config.toml, or pick several
interactively (a filterable checklist) when no id is given. Only entries in
config.toml can be removed here: an entry a component fragment under conf.d/
declares is managed by dotty init — deselect the component instead. Nothing
is uninstalled: removed entries stay on the machine until
`dotty packages sync` removes what the profile no longer declares —
pass --sync to run it immediately.

```
dotty packages remove [--sync] [<id> ...] [flags]
```

### Examples

```
  dotty packages remove aqua:sharkdp/bat brew-cask:raycast
  dotty packages rm
  dotty packages remove --sync brew:colima
```

### Options

```
  -h, --help                       help for remove
      --sync dotty packages sync   run dotty packages sync after removing
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty packages](dotty_packages.md)	 - Manage the profile's packages (mise tools and bootstrap packages).

