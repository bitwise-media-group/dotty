## dotty packages edit

Open the profile's config.toml in the default editor.

### Synopsis

Open the profile's user-owned mise config.toml in $VISUAL / $EDITOR. The
component fragments under conf.d/ are re-rendered by dotty init and are not
the place for edits. With --sync or --upgrade, the corresponding command
runs after the editor exits.

```
dotty packages edit [--sync | --upgrade] [flags]
```

### Examples

```
  dotty packages edit
  dotty packages edit --sync
```

### Options

```
  -h, --help                             help for edit
      --sync dotty packages sync         run dotty packages sync after editing
      --upgrade dotty packages upgrade   run dotty packages upgrade after editing
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty packages](dotty_packages.md)	 - Manage the profile's packages (mise tools and bootstrap packages).

