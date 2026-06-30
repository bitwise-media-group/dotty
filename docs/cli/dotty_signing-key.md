## dotty signing-key

Create and use SSH signing keys on hardware security keys.

### Synopsis

Signing keys are resident FIDO2 credentials on a YubiKey, used to sign
git commits, tags, and files via ssh-keygen. dotty keeps only key-handle
stubs on disk (under the private $XDG_DATA_HOME/dotty/security-key) — the
secret never leaves the hardware. Keys are PIN-protected (verify-required)
and need no touch per signature.

### Examples

```
  dotty signing-key new
  dotty signing-key list
  dotty signing-key get --security-key=work
  dotty signing-key sign --print-git-config
```

### Options

```
  -h, --help                  help for signing-key
      --security-key string   security key to use: a serial number or an alias
      --username string       username the key is enrolled under (default: the current user)
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
```

### SEE ALSO

* [dotty](dotty.md)	 - Utilities for a terminal-driven workflow and dotfiles.
* [dotty signing-key authorize](dotty_signing-key_authorize.md)	 - Authorize a signing key for SSH login on a remote host.
* [dotty signing-key get](dotty_signing-key_get.md)	 - Print a signing key's stub and public key.
* [dotty signing-key import](dotty_signing-key_import.md)	 - Import existing SSH signing-key stubs that match a connected security key.
* [dotty signing-key list](dotty_signing-key_list.md)	 - List the signing keys on plugged-in security keys.
* [dotty signing-key new](dotty_signing-key_new.md)	 - Create a resident SSH signing key on a security key.
* [dotty signing-key sign](dotty_signing-key_sign.md)	 - Sign a payload with a signing key (ssh-keygen proxy).

