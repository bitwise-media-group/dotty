## dotty security-key add

Add a named alias for a security key.

### Synopsis

Register a memorable alias for a security key's serial number. Without
--serial, the plugged-in key is used (or picked from a list when several are
present, with the option to type a serial by hand). Alias names are unique
across all keys. Without --description, dotty offers to open $EDITOR for one;
that step can be skipped.

```
dotty security-key add [--name=<name>] [--description=<description>] [flags]
```

### Examples

```
  dotty security-key add
  dotty security-key --serial=12345678 add --name=work
  dotty sk add --name=backup --description="kept in the safe"
```

### Options

```
      --description string   what this key is for
  -h, --help[=true|false]    help for add
      --name string          alias name (unique across all keys)
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --serial string    serial number of the security key
```

### SEE ALSO

* [dotty security-key](dotty_security-key.md)	 - Manage hardware security keys.

