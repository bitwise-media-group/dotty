## dotty packages lock

Refresh mise.lock for every platform.

### Synopsis

Resolve every tool the profile declares for every platform it publishes and
write the versions, URLs, and checksums to the profile's mise.lock — the one
committed lockfile serves macOS and Linux. add, sync, and upgrade do this
on their own; lock is for after editing config.toml by hand.

```
dotty packages lock [flags]
```

### Examples

```
  dotty packages lock
```

### Options

```
  -h, --help   help for lock
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty packages](dotty_packages.md)	 - Manage the profile's packages (mise tools and bootstrap packages).

