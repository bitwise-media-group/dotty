## dotty private

Manage the encrypted private dotfiles repository.

### Synopsis

The private repository carries what the public one must not: git identities,
ssh host configuration, known_hosts — anything that leaks PII. Each profile
keeps its own home tree there, age-encrypted to that profile's security keys
alone, so the repository stores ciphertext only and a leak exposes nothing.
Decrypted files live under the dotty data directory (0700) and reach $HOME
through an active-profile symlink, so activating another profile swaps the
whole private identity at once.

### Examples

```
  dotty private init ~/Repos/dotfiles.private
  dotty private enroll --serial 17741369
  dotty private encrypt ~/.ssh/known_hosts
  dotty private status
```

### Options

```
  -h, --help          help for private
      --repo string   private repository (default: the active profile's stored answer)
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty private edit](dotty_private_edit.md)	 - Edit an encrypted entry in place.
* [dotty private encrypt](dotty_private_encrypt.md)	 - Adopt a live file into the private profile, encrypted.
* [dotty private enroll](dotty_private_enroll.md)	 - Add a security key to the profile's recipients.
* [dotty private init](dotty_private_init.md)	 - Scaffold or adopt the private repository.
* [dotty private link](dotty_private_link.md)	 - Decrypt the profile's private tree and link it into $HOME.
* [dotty private rekey](dotty_private_rekey.md)	 - Re-encrypt every entry to the profile's current recipients.
* [dotty private status](dotty_private_status.md)	 - Report each private entry's freshness, plus repository hygiene.
* [dotty private verify](dotty_private_verify.md)	 - Check the repository for plaintext accidents.

