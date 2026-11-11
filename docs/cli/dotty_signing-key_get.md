## dotty signing-key get

Print a signing key's stub and public key.

### Synopsis

Print the private key stub and public key for a username on a security
key. --format=key prints a single key::<public-key> line for git's
gpg.ssh.defaultKeyCommand; that mode never prompts (git captures the output),
preferring the ed25519 key when a user has several types enrolled.

```
dotty signing-key get [--format=<text|key>] [flags]
```

### Examples

```
  dotty signing-key get
  dotty signing-key get --security-key=work --username=deavon
  dotty signing-key get --format=key   # for gpg.ssh.defaultKeyCommand
```

### Options

```
      --format string       output format: text (stub + public key) or key (git literal key line) (default "text")
  -h, --help[=true|false]   help for get
```

### Options inherited from parent commands

```
      --profile string        profile to operate on (defaults to the active profile)
      --security-key string   security key to use: a serial number or an alias
      --username string       username the key is enrolled under (default: the current user)
```

### SEE ALSO

* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.

