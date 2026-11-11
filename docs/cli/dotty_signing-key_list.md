## dotty signing-key list

List the signing keys on plugged-in security keys.

### Synopsis

Show the signing keys of all currently plugged-in YubiKeys in a
fuzzy-filterable table (serial, aliases, key type, username). Selecting a row
prints its private key stub and public key; esc exits without printing.
Unlike the other signing-key verbs, list never asks you to pick a key first.
Without a terminal the table prints plainly and nothing is selectable.

```
dotty signing-key list [flags]
```

### Examples

```
  dotty signing-key list
  dotty signing-key list --username=deavon
  dotty ssh-key list --security-key=work
```

### Options

```
  -h, --help[=true|false]   help for list
```

### Options inherited from parent commands

```
      --profile string        profile to operate on (defaults to the active profile)
      --security-key string   security key to use: a serial number or an alias
      --username string       username the key is enrolled under (default: the current user)
```

### SEE ALSO

* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.

