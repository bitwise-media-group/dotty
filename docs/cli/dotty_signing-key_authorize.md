## dotty signing-key authorize

Authorize a signing key for SSH login on a remote host.

### Synopsis

Append a signing key's public key to a remote host's authorized_keys so the
YubiKey can log in there over SSH. Only keys on a currently plugged-in YubiKey
are offered; a single match is used directly, otherwise dotty shows a
fuzzy-filterable picker. Narrow the choice up front with --security-key and
--username.

dotty connects with your own ssh client, so ~/.ssh/config, the agent,
known_hosts, and whatever auth the host already accepts all apply — you must be
able to reach the host now to enrol the key for later. The authorized_keys file
is only appended to, never rewritten; ~/.ssh (0700) and the file (0600) are
created if missing. Authorizing a key already on file is an error and changes
nothing.

The entry is prefixed with --options (default no-touch-required): dotty's keys
are enrolled no-touch-required, and sshd rejects a no-touch signature unless the
authorized_keys line says so. Extend it for more control — e.g.
--options=no-touch-required,verify-required to also demand the FIDO PIN, or pass
--options="" to write a bare key.

```
dotty signing-key authorize <[user@]host> [flags]
```

### Examples

```
  dotty signing-key authorize deavon@server
  dotty signing-key authorize --security-key=work root@host
  dotty signing-key authorize --path=/etc/ssh/keys/authorized_keys admin@host
```

### Options

```
  -h, --help             help for authorize
      --options string   authorized_keys option list to prefix (comma-separated; empty for none) (default "no-touch-required")
      --path string      remote authorized_keys file to append to (default "~/.ssh/authorized_keys")
```

### Options inherited from parent commands

```
      --profile string        profile to operate on (defaults to the active profile)
      --security-key string   security key to use: a serial number or an alias
      --username string       username the key is enrolled under (default: the current user)
```

### SEE ALSO

* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.

