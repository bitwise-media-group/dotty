## dotty signing-key trust

Trust plugged-in keys' signatures in the git allowed_signers file.

### Synopsis

Append every signing key on the plugged-in YubiKeys to your OpenSSH
allowed_signers file, so git can verify the commits and tags they sign. Each
entry pairs your committer email (git config user.email) with the key:

  you@example.com sk-ssh-ed25519@openssh.com AAAA...

The file comes from git's gpg.ssh.allowedSignersFile, falling back to
~/.ssh/allowed_signers (dotty then prints the one-liner to point git at it).
~/.ssh (0700) and the file (0600) are created if missing; existing entries are
kept and a key already trusted for your email is left alone, so re-running is
safe. --security-key and --username narrow which stubs are added; --path writes
a different file.

```
dotty signing-key trust [flags]
```

### Examples

```
  dotty signing-key trust
  dotty signing-key trust --username=deavon
  dotty signing-key trust --path=~/.config/git/allowed_signers
```

### Options

```
  -h, --help[=true|false]   help for trust
      --path string         allowed_signers file to write (default: git gpg.ssh.allowedSignersFile, else ~/.ssh/allowed_signers)
```

### Options inherited from parent commands

```
      --profile string        profile to operate on (defaults to the active profile)
      --security-key string   security key to use: a serial number or an alias
      --username string       username the key is enrolled under (default: the current user)
```

### SEE ALSO

* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.

