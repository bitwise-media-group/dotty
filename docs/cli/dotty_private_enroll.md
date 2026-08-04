## dotty private enroll

Add a security key to the profile's recipients.

### Synopsis

Create an age identity on a YubiKey (a PIV retired slot, via
age-plugin-yubikey), commit its non-secret identity stub beside the
profile's recipients, and add the recipient — from then on every encrypt
targets the key, and any machine with it plugged in decrypts. The PIV
standard slots (smart-card login, signing) are never touched, but the PIV
PIN and its retry counter are shared with them: three wrong PINs at decrypt
time lock PIV login too.

The default policies cost one PIN per PIV session and one touch per
15-second window (pin-policy once, touch-policy cached), so a whole-tree
decrypt is a single PIN+touch burst. Enrolling a second key onto a profile
that already has ciphertext prompts a rekey — existing files never learn
about a new recipient on their own.

```
dotty private enroll [flags]
```

### Examples

```
  dotty private enroll
  dotty private enroll --serial 17741369
  dotty private enroll --security-key=backup --touch-policy=always
```

### Options

```
  -h, --help                  help for enroll
      --pin-policy string     PIV PIN policy: always, once, or never (default "once")
      --security-key string   security-key alias (or serial) to enroll
      --serial string         YubiKey serial to enroll
      --slot int              identity slot 1-20 (default: first free)
      --touch-policy string   touch policy: always, cached (15s), or never (default "cached")
```

### Options inherited from parent commands

```
      --profile string   profile to operate on (defaults to the active profile)
      --repo string      private repository (default: the active profile's stored answer)
```

### SEE ALSO

* [dotty private](dotty_private.md)	 - Manage the encrypted private dotfiles repository.

