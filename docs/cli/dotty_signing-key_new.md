## dotty signing-key new

Create a resident SSH signing key on a security key.

### Synopsis

Enroll a new resident, PIN-protected (verify-required, no-touch-required)
SSH key on a YubiKey via ssh-keygen, filing the key-handle stub under the
key's serial in the private data directory.

With several YubiKeys plugged in, dotty asks you to unplug and re-insert the
intended key — the only reliable way to map a serial to a FIDO device, since
YubiKeys expose no USB serial. Esc falls back to a picker; ssh-keygen's own
touch-select then chooses the hardware, so touch the key whose serial dotty
names (it is etched on the key).

Re-enrolling an existing username replaces the resident credential on the
device as well as the stub.

```
dotty signing-key new [--type=<ed25519|ecdsa>] [flags]
```

### Examples

```
  dotty signing-key new
  dotty signing-key new --security-key=work --type=ecdsa
  dotty signing-key new --username=deavon
```

### Options

```
  -h, --help[=true|false]   help for new
      --type string         key type: ed25519 or ecdsa (default "ed25519")
```

### Options inherited from parent commands

```
      --profile string        profile to operate on (defaults to the active profile)
      --security-key string   security key to use: a serial number or an alias
      --username string       username the key is enrolled under (default: the current user)
```

### SEE ALSO

* [dotty signing-key](dotty_signing-key.md)	 - Create and use SSH signing keys on hardware security keys.

