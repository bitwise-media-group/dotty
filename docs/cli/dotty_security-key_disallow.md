## dotty security-key disallow

Remove security keys from a profile's allowlist.

### Synopsis

Remove security keys from a profile's allowlist (the active profile unless
--profile names another). Removing the last entry lifts the restriction —
the profile allows every key again.

```
dotty security-key disallow [<serial>|<alias>...] [flags]
```

### Examples

```
  dotty security-key disallow
  dotty --profile=work security-key disallow 12345678
```

### Options

```
  -h, --help[=true|false]   help for disallow
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --serial string    serial number of the security key
```

### SEE ALSO

* [dotty security-key](dotty_security-key.md)	 - Manage hardware security keys.

