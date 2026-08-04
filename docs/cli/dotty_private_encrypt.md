## dotty private encrypt

Adopt a live file into the private profile, encrypted.

### Synopsis

Encrypt a file from the home directory to the profile's recipients and
store the ciphertext in the private repository at the matching home-relative
path. The plaintext also lands in the decrypted private area so the entry is
immediately linkable, and the manifest records both hashes. Re-encrypting an
already-adopted entry is how a local edit (status: drifted) gets committed
back.

```
dotty private encrypt <path> [flags]
```

### Examples

```
  dotty private encrypt ~/.ssh/known_hosts
  dotty private encrypt .config/private/git/config
```

### Options

```
  -h, --help   help for encrypt
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      private repository (default: the active profile's stored answer)
```

### SEE ALSO

* [dotty private](dotty_private.md)	 - Manage the encrypted private dotfiles repository.

