## dotty signing-key import

Import existing SSH signing-key stubs that match a connected security key.

### Synopsis

Import key-handle stubs from elsewhere on disk into the private data
directory. Each stub is verified against the connected YubiKey(s): dotty
downloads the resident credentials from the hardware with ssh-keygen -K (which
prompts for the FIDO PIN and a touch) and keeps only the stubs whose public key
is actually resident on a connected key, filing each under that key's serial.
Stubs that match no connected key are skipped with a warning; if nothing
matches, import fails.

<path> may be a single stub file or a directory, which is walked recursively so
both flat and <serial>/id_*_sk_* layouts work. The secret never leaves the
hardware — only the key-handle stub and its public key are copied.

With several YubiKeys connected, dotty asks you to touch each in turn; touch the
key whose serial it names (etched on the key). --security-key narrows the import
to one connected key. Aliases are not set here — add them with
`dotty security-key add` afterwards.

```
dotty signing-key import <path> [--rm] [flags]
```

### Examples

```
  dotty signing-key import ./backup
  dotty signing-key import ./backup --rm
  dotty signing-key import --security-key=work ~/keys/id_ed25519_sk_deavon
```

### Options

```
  -h, --help[=true|false]   help for import
      --rm[=true|false]     remove the source stub files after a successful import
```

### Options inherited from parent commands

```
      --profile string        profile to operate on (defaults to the active profile)
      --security-key string   security key to use: a serial number or an alias
      --username string       username the key is enrolled under (default: the current user)
```

### SEE ALSO

* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.

