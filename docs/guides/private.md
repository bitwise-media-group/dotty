<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Private dotfiles, encrypted

The public dotfiles repository must never carry PII: git identities, ssh host
blocks, `known_hosts`, `allowed_signers` — anything that says who you are or
what your machines are called. [`dotty private`](../cli/dotty_private.md)
manages a **second repository** for exactly that content, storing every secret
as [age](https://age-encryption.org) ciphertext encrypted to your YubiKeys. The
repository holds ciphertext only, so even a leak of the repo (or the GitHub
account) exposes nothing.

## The model

```text
dotfiles.private/
├── .dotty-private                  # marker; keeps dotfiles verbs from misrouting
└── profiles/
    ├── personal/
    │   ├── age/
    │   │   ├── recipients.txt      # encrypt targets: this profile's keys only
    │   │   └── identity-<serial>.txt   # non-secret stubs, committed
    │   └── home/                   # mirrors $HOME; <path>.age = encrypted
    │       ├── .config/private/git/config.age
    │       ├── .ssh/config.d/personal.conf.age
    │       └── .ssh/known_hosts.age
    └── work/…
```

Private profiles mirror the public repository's profiles by name, and each is
encrypted **only to its own keys** — the work profile's YubiKey cannot open the
personal profile's files. A profile with several enrolled keys (a backup key)
encrypts to all of them, and whichever is plugged in decrypts.

Decryption happens at link time, into `~/.local/share/dotty/private/<profile>/`
(`0700`, files `0600`). `$HOME` symlinks route through
`~/.local/share/dotty/private/active-profile`, so
[`dotty profile activate`](../cli/dotty_profile_activate.md) retargets one
symlink and your whole private identity — git author included — swaps atomically
with the profile. Plaintext never enters the repository working tree: encryption
reads stdin, edits happen in the `0700` data area, and a scaffolded pre-commit
hook runs [`dotty private verify`](../cli/dotty_private_verify.md) against
accidents.

## Setup

```sh
dotty private init ~/Repos/dotfiles.private   # scaffold + record in the profile
dotty private enroll                          # age identity on the plugged-in YubiKey
dotty private encrypt ~/.ssh/known_hosts      # adopt files, one by one
dotty private link                            # decrypt + symlink into $HOME
```

`enroll` uses [age-plugin-yubikey](https://github.com/str4d/age-plugin-yubikey)
(both come from the security-keys Brewfile fragment): the identity lands in a
PIV **retired** slot, so smart-card login and your SSH signing keys are
untouched. Two caveats worth knowing:

- The **PIV PIN and its retry counter are shared** with smart-card login. Three
  wrong PINs at decrypt time lock PIV login too.
- The default policies (`--pin-policy=once`, `--touch-policy=cached`) make a
  whole-tree decrypt cost one PIN and one touch. The PIN session ends when
  another applet is used — an ssh signature (FIDO2) between decrypts brings the
  prompt back.

Enroll a second key onto the same profile for redundancy, then run
[`dotty private rekey`](../cli/dotty_private_rekey.md) so existing ciphertext
learns the new recipient. The identity stubs are not secrets — the plugin
regenerates them from the token — which is why they commit with the repo.

## Day to day

```sh
dotty private status              # ok / stale / drifted / conflict / missing
dotty private edit .config/private/git/config
dotty private link                # after a git pull; only changed files decrypt
```

Decryption is incremental: a manifest of content hashes keeps steady-state
relinks from ever touching the hardware, and local edits are never overwritten —
a `drifted` file wants `dotty private encrypt`, a `conflict` wants a human. With
no YubiKey plugged in, `link` keeps the previous plaintext and warns (`--strict`
to fail instead), so a work machine without the personal key simply skips the
personal profile.

`dotty init` asks for the private repository (stored per profile as
`privateRepo`) and links it automatically right before the git-identity step — a
machine restored from scratch needs the public repo, the private repo, and one
YubiKey.

## Splitting identities

The private git config is per profile, so what used to be one `includeIf`
monolith splits along machine classes: the personal profile's
`.config/private/git/config.age` carries the personal `[user]` block and its
host routing, the work profile's carries the corporate identity. Verify a swap
with:

```sh
dotty profile activate --name=work && git var GIT_COMMITTER_IDENT
```

For ssh, the public template's `~/.ssh/config` now starts with
`Include ~/.ssh/config.d/*.conf`; private host blocks belong in
`profiles/<name>/home/.ssh/config.d/<name>.conf.age`. First match wins in ssh,
so the drop-ins override the public defaults, and the glob matching nothing is
fine on machines without private files.
