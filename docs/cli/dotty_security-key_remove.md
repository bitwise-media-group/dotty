## dotty security-key remove

Remove security key aliases.

### Synopsis

Remove one alias by --name, or pick aliases interactively: a tree grouped
by serial number — collapsible with h/l, filterable with / — where space
selects and enter confirms. A key may carry several aliases, so multiple
selections are removed in one go.

```
dotty security-key remove [--name=<name>] [flags]
```

### Examples

```
  dotty security-key remove --name=old-key
  dotty security-key remove
  dotty sk --serial=12345678 rm
```

### Options

```
  -h, --help[=true|false]   help for remove
      --name string         alias to remove
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --serial string    serial number of the security key
```

### SEE ALSO

* [dotty security-key](dotty_security-key.md)	 - Manage hardware security keys.

