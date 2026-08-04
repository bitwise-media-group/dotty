## dotty private link

Decrypt the profile's private tree and link it into $HOME.

### Synopsis

Materialize the active profile's plaintext under the dotty data directory
(0700) and symlink it over $HOME through the private active-profile
indirection, so activating another profile swaps every private file at once.
Decryption is incremental — only entries whose ciphertext changed are
decrypted, so a steady-state relink never touches the security key. Entries
that cannot be decrypted (key unplugged) or that carry unreconciled local
edits are skipped with a warning and the previous copy stays usable;
--strict turns any skip into a failure. Existing real files at link sites
resolve per --on-conflict; by default dotty asks (backing up when there is
no terminal).

```
dotty private link [flags]
```

### Examples

```
  dotty private link
  dotty private link --on-conflict=backup --strict
```

### Options

```
  -h, --help                 help for link
      --on-conflict string   existing-file resolution: backup, adopt, skip, or fail (default: ask; backup when not a terminal)
      --strict               fail on any entry that cannot be decrypted or conflicts
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      private repository (default: the active profile's stored answer)
```

### SEE ALSO

* [dotty private](dotty_private.md)	 - Manage the encrypted private dotfiles repository.

