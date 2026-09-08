## dotty packages upgrade

Upgrade every package the profile declares.

### Synopsis

Move every tool and bootstrap package to its newest version without removing
anything — mise upgrade, then mise bootstrap packages upgrade — and refresh
mise.lock for every platform so the new versions travel with the profile.

```
dotty packages upgrade [flags]
```

### Examples

```
  dotty packages upgrade
  dotty --profile=work packages upgrade
```

### Options

```
  -h, --help   help for upgrade
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty packages](dotty_packages.md)	 - Manage the profile's packages (mise tools and bootstrap packages).

