## dotty private rekey

Re-encrypt every entry to the profile's current recipients.

### Synopsis

Decrypt and re-encrypt all of the profile's ciphertext against the current
recipients file. Run it after enrolling a new security key — existing files
never learn about a recipient on their own — or after removing one, so the
departed key stops opening future revisions (it can still open anything
already in the git history; rotate the content itself when that matters).
Needs one currently-enrolled key plugged in.

```
dotty private rekey [flags]
```

### Examples

```
  dotty private rekey
  dotty private rekey --profile work
```

### Options

```
  -h, --help   help for rekey
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      private repository (default: the active profile's stored answer)
```

### SEE ALSO

* [dotty private](dotty_private.md)	 - Manage the encrypted private dotfiles repository.

