## dotty packages sync

Make the machine match the profile's packages exactly.

### Synopsis

Synchronise the machine with the profile: refresh mise.lock for every
platform, install the locked tools and the bootstrap packages that are
missing, prune tool versions the profile no longer references, and remove
the bootstrap packages it no longer declares. When bootstrap packages would be
removed, dotty shows the list and asks first unless --force is set. Cask
removal is conservative: mise only removes casks it installed itself, so an
app installed by hand may stay put.

```
dotty packages sync [--force] [flags]
```

### Examples

```
  dotty packages sync
  dotty packages sync --force
```

### Options

```
      --force   remove undeclared packages without asking
  -h, --help    help for sync
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty packages](dotty_packages.md)	 - Manage the profile's packages (mise tools and bootstrap packages).

