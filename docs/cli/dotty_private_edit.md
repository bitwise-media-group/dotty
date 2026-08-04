## dotty private edit

Edit an encrypted entry in place.

### Synopsis

Decrypt one entry to a 0600 scratch file inside the private data area
(never a shared temp directory), open it in $VISUAL/$EDITOR, and encrypt the
result back to the profile's recipients. Unchanged content is left alone —
age output is not deterministic, so re-encrypting an identical file would
only manufacture a spurious diff. A plaintext (unencrypted) entry opens
directly in the repository.

```
dotty private edit <path> [flags]
```

### Examples

```
  dotty private edit .config/private/git/config
  dotty private edit ~/.ssh/config.d/personal.conf
```

### Options

```
  -h, --help   help for edit
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      private repository (default: the active profile's stored answer)
```

### SEE ALSO

* [dotty private](dotty_private.md)	 - Manage the encrypted private dotfiles repository.

