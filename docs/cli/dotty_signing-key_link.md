## dotty signing-key link

Symlink a stable path at the plugged-in key's stub, for ssh.

### Synopsis

Point a stable symlink (and its .pub sibling) at the resolved signing
key's stub, then print the link path. ssh can then name one fixed IdentityFile
that always follows whichever YubiKey is plugged in. With no argument the link
is ~/.ssh/id_sk_current; pass a path to place it elsewhere. Like git's callout
it never prompts; when several keys are connected, narrow with
--security-key/--username. Prefers the ed25519 key when a user has several types
enrolled. Drive it from ssh's Match exec so the right identity is selected on
connect:

  Match host github.com exec "dotty signing-key link >/dev/null"
      IdentityFile ~/.ssh/id_sk_current
      IdentitiesOnly yes
      IdentityAgent none

```
dotty signing-key link [path] [flags]
```

### Examples

```
  dotty signing-key link
  dotty signing-key link ~/.ssh/id_sk_work
  dotty signing-key link --security-key=work
```

### Options

```
  -h, --help   help for link
```

### Options inherited from parent commands

```
      --profile string        profile to operate on (defaults to the active profile)
      --security-key string   security key to use: a serial number or an alias
      --username string       username the key is enrolled under (default: the current user)
```

### SEE ALSO

* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.

