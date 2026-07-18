<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Signing keys & first commit

dotty signs git commits with **SSH keys resident on a YubiKey**: the private key
is generated on the hardware and never leaves it. What lands on disk is a key
_stub_ — useless without the physical key and a touch. This page enrols a key,
wires git up, and closes the loop `init` left open: the repo's first, signed
commit.

If you answered "no" to security keys in the wizard, skip ahead to
[Coding agents & hardening](agents.md).

## Name your security key

```sh
dotty security-key add --name=work
```

[`dotty security-key add`](../cli/dotty_security-key_add.md) maps the plugged-in
key's serial to an alias, so later commands can say `--security-key=work`
instead of a serial number. Aliases are stored in the private data directory,
and a profile can restrict which keys it allows with
[`security-key allow`](../cli/dotty_security-key_allow.md) — useful when the
work profile must only use employer-issued keys.

## Create a resident signing key

```sh
dotty signing-key new
```

[`dotty signing-key new`](../cli/dotty_signing-key_new.md) enrols a resident
FIDO2 SSH key (ed25519 by default) on the YubiKey — touch it when it blinks. The
public stub lands in `$XDG_DATA_HOME/dotty` (mode 0700).

## Wire git up

If you ran `init` with security keys enabled, this is **already done**: the
active profile's rendered `git.gitconfig` sets `gpg.format=ssh`, routes signing
through dotty, and resolves the key at commit time with
`dotty signing-key get --format=key`. For any other machine or a non-dotty
setup, print the exact config to paste:

```sh
dotty signing-key sign --print-git-config
```

## Trust yourself, and make the first commit

git verifies signatures against an `allowed_signers` file;
[`dotty signing-key trust`](../cli/dotty_signing-key_trust.md) appends your key
to it. Then commit the repository `init` staged:

```sh
dotty signing-key trust
cd ~/Repos/dotfiles
git commit -m "chore: initial scaffold from dotty"
git log --show-signature -1
```

The YubiKey blinks; touch it, and `git log --show-signature` shows a good
signature. Your dotfiles history starts verified.

??? note "PIN prompts, stable paths, and SSH logins"

    - **PIN prompts** — OpenSSH asks for the FIDO2 PIN on the terminal,
      which breaks under GUI apps and agents. `init` links a
      `dotty-ssh-askpass` applet into the data directory and exports
      `SSH_ASKPASS`, so PIN prompts route to **pinentry-mac** — graphical,
      with optional Keychain caching (see the `gpg-keychain`
      [defaults group](../reference/macos-defaults.md#gpg-keychain)).
    - **A stable key path** —
      [`dotty signing-key link`](../cli/dotty_signing-key_link.md)
      maintains `~/.ssh/id_sk_current` pointing at the active profile's
      allowed key. The scaffolded SSH config runs it on every connection
      (`Match host * exec`), so `IdentityFile` never changes even when
      you swap keys or profiles.
    - **SSH logins** —
      [`dotty signing-key authorize`](../cli/dotty_signing-key_authorize.md)
      appends the key to a remote host's `authorized_keys`, with
      `no-touch-required` as an option.

## More keys, more machines

```sh
dotty signing-key list                    # every enrolled key
dotty signing-key get                     # the active key's public half
dotty signing-key import ~/backup/id_sk   # adopt an existing stub
```

A second machine needs no re-enrolment: resident keys travel with the hardware.
Plug the YubiKey in, [`import`](../cli/dotty_signing-key_import.md) or
regenerate the stub, and the same key signs there too.

[Next: Coding agents & hardening →](agents.md)
