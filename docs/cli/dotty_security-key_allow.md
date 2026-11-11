## dotty security-key allow

Restrict a profile to specific security keys.

### Synopsis

Add security keys to a profile's allowlist. Once a profile has one, its
machines refuse every other key for signing, linking, enrollment, and import
— so personal keys are never used against work devices, and vice versa.

The list applies to the active profile unless --profile names another. It is
a property of the machine class: it lives in the profile's profile.json,
travels with the dotfiles repository, and activating another profile swaps
it. Arguments are serials or aliases; without arguments, an interactive
picklist offers the known and connected keys. Removing every entry later
(dotty security-key disallow) lifts the restriction.

```
dotty security-key allow [<serial>|<alias>...] [flags]
```

### Examples

```
  dotty security-key allow            # pick from known + connected keys
  dotty security-key allow work-key
  dotty --profile=work security-key allow 12345678
```

### Options

```
  -h, --help[=true|false]   help for allow
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --serial string    serial number of the security key
```

### SEE ALSO

* [dotty security-key](dotty_security-key.md)	 - Manage hardware security keys.

