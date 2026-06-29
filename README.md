# dotty

Utilities for a terminal-driven workflow and dotfiles: system profiles that
travel across machines, reproducible Homebrew Brewfiles, named aliases for
hardware security keys, and SSH signing keys that live on YubiKeys — including
git commit signing.

Every command follows `dotty <noun> <verb>`:

```sh
dotty profile new --name=work        # create a profile (and offer to activate it)
dotty profile activate               # fuzzy-pick and activate a profile
dotty brewfile add --cask ghostty    # add to the Brewfile and install
dotty brewfile sync                  # make the machine match the Brewfile
dotty security-key add --name=work   # alias a YubiKey serial
dotty signing-key new                # enroll a resident SSH signing key
```

The generated command reference lives in [docs/cli](docs/cli/dotty.md).

## Install

```sh
brew install bitwise-media-group/tap/dotty
```

or with the Go toolchain:

```sh
go install github.com/bitwise-media-group/dotty/cmd@latest
```

or download an archive from the
[releases page](https://github.com/bitwise-media-group/dotty/releases). External
tools dotty drives: `brew` (with `brew bundle`), `ykman`, `fido2-token`
(libfido2), and `ssh-keygen` (OpenSSH 8.2+ with FIDO support).

## Where things live

dotty splits its state for privacy:

- **`$XDG_CONFIG_HOME/dotty`** (`~/.config/dotty`) — profiles and the
  `active-profile` symlink. Shareable configuration, safe for a public dotfiles
  repository.
- **`$XDG_DATA_HOME/dotty`** (`~/.local/share/dotty`) — security-key aliases and
  signing-key stubs, created `0700`/`0600`. PII-adjacent; manage it from a
  private repository if you sync it at all. The stubs are FIDO2 key handles, not
  private keys — the secrets never leave the hardware, and resident keys can be
  re-downloaded with `ssh-keygen -K`.

## Git commit signing

Enroll a key, then print the ready-to-paste configuration:

```sh
dotty signing-key new
dotty signing-key sign --print-git-config
```

That configures `gpg.format=ssh`, points `gpg.ssh.program` at dotty, and uses
`dotty signing-key get --format=key` as `gpg.ssh.defaultKeyCommand`. Signing a
commit then costs one PIN entry — the key was enrolled `verify-required` +
`no-touch-required`, and ssh-keygen locates the right YubiKey from the key
handle automatically.

Static alternative (no dotty on the signing path): set `user.signingKey` to the
stub path printed by `dotty signing-key get`.

## Development

```sh
make pr    # the full local gate: tidy license fmt lint test fuzz build docs snapshot
```

Go developer CLIs are pinned in `tools/go.mod` and run via
`go tool -modfile=tools/go.mod`; Node tooling is pinned in `package.json`. CI,
security analysis, and releases run through the org's reusable GitHub Actions
workflows
([bitwise-media-group/github-workflows](https://github.com/bitwise-media-group/github-workflows)).
Releases are driven by
[release-please](https://github.com/googleapis/release-please): merging its
release PR cuts the `vX.Y.Z` tag and draft release, then GoReleaser publishes
signed archives, checksums, SBOMs, and the Homebrew cask.
