## dotty signing-key sign

Sign a payload with a signing key (ssh-keygen proxy).

### Synopsis

Proxy to ssh-keygen -Y sign using a hardware-backed signing key. Built for
git: point gpg.ssh.program at dotty (or a dotty-ssh-sign symlink) and git's
own arguments pass straight through; when git supplies a literal public key
via gpg.ssh.defaultKeyCommand, dotty swaps in the matching stub so the
YubiKey signs. Run with --print-git-config for ready-to-paste setup.

Humans can sign files too: with no -f, dotty resolves the key from
--security-key/--username (or the single plugged-in YubiKey) and defaults
the namespace to "file".

```
dotty signing-key sign [ssh-keygen args] [file ...] [flags]
```

### Examples

```
  dotty signing-key sign --print-git-config
  dotty signing-key sign document.txt
  dotty signing-key sign --security-key=work -n release artifact.tar.gz
```

### Options

```
  -h, --help[=true|false]   help for sign
```

### Options inherited from parent commands

```
      --profile string        profile to operate on (defaults to the active profile)
      --security-key string   security key to use: a serial number or an alias
      --username string       username the key is enrolled under (default: the current user)
```

### SEE ALSO

* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.

