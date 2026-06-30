<!--
  Copyright 2026 Bitwise Media Group Ltd
  SPDX-License-Identifier: MIT
-->

# Installation

The recommended way to install dotty is the [Homebrew tap](#homebrew). The
alternative methods below all install the same signed binary, and every release
ships checksums, keyless [cosign](#signatures) signatures, and a SLSA
build-provenance [attestation](#attestations) you can verify yourself.

## Homebrew

dotty publishes a cask to
[`bitwise-media-group/homebrew-tap`](https://github.com/bitwise-media-group/homebrew-tap).
The tap is tapped automatically the first time you reference it:

```sh
brew install bitwise-media-group/tap/dotty
```

The cask also installs the man pages and shell completions, and strips the macOS
quarantine attribute so the binary runs without a Gatekeeper prompt. Upgrade and
uninstall the usual way:

```sh
brew upgrade dotty
brew uninstall dotty
```

## Go install

With the Go toolchain (Go 1.24+):

```sh
go install github.com/bitwise-media-group/dotty/cmd@latest
```

!!! note "go install builds from source"

    A `go install` build is compiled on your machine, so it carries no release
    version stamp and is not covered by the cosign signature or attestation
    below. Use the Homebrew cask or a release archive when you want a verifiable
    artifact.

## Manually

Download the archive for your platform from the
[releases page](https://github.com/bitwise-media-group/dotty/releases), extract
it, and move the `dotty` binary onto your `PATH`. Archives are named
`dotty_<version>_<os>_<arch>.tar.gz` and contain the binary, the `LICENSE`, and
the man pages.

```sh
# e.g. macOS on Apple Silicon
tar -xzf dotty_<version>_darwin_arm64.tar.gz
install -m 0755 dotty /usr/local/bin/dotty
```

`<os>` is `darwin` or `linux`; `<arch>` is `amd64` or `arm64`.

## Verifying the artifacts

Every release attaches a `checksums.txt`, a cosign signature bundle per binary,
SPDX SBOMs, and a GitHub build-provenance attestation. None of the steps below
require trusting a long-lived key — cosign and `gh` verify against Sigstore's
transparency log and GitHub's attestation API.

### Checksums

`checksums.txt` lists the SHA-256 of every release archive. Download it
alongside the archives you grabbed and check them:

```sh
sha256sum --ignore-missing -c checksums.txt
```

On macOS without the GNU coreutils, use
`shasum -a 256 --ignore-missing -c checksums.txt`.

### Signatures

Each binary is signed keyless with [cosign](https://docs.sigstore.dev/) in the
release workflow; the signature travels as a Sigstore bundle named
`dotty_<os>_<arch>.sigstore.json` on the release. Extract the binary from its
archive, download the matching bundle, then verify the binary against it:

```sh
cosign verify-blob \
  --certificate-identity-regexp '^https://github.com/bitwise-media-group/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --bundle dotty_darwin_arm64.sigstore.json \
  dotty
```

!!! info "Why a regexp for the identity"

    dotty signs from the organisation's
    [reusable release workflow](https://github.com/bitwise-media-group/github-workflows),
    which is pinned — and bumped — by commit SHA, so the certificate's exact
    identity URL changes between releases. The
    `^https://github.com/bitwise-media-group/` regexp pins the signer to the
    organisation while staying stable across those bumps. The OIDC issuer is
    always GitHub Actions, so it is matched exactly.

### Attestations

The release workflow records a
[SLSA build-provenance attestation](https://docs.github.com/en/actions/security-guides/using-artifact-attestations)
over everything in `checksums.txt`. Verify an archive with the GitHub CLI — no
download of the attestation needed, `gh` fetches it from the API:

```sh
gh attestation verify dotty_<version>_darwin_arm64.tar.gz \
  --repo bitwise-media-group/dotty
```

### SBOMs

An SPDX software bill of materials is attached for each archive as
`dotty_<version>_<os>_<arch>.tar.gz.sbom.json`. Inspect it with any SPDX-aware
tool, for example:

```sh
grype sbom:dotty_<version>_darwin_arm64.tar.gz.sbom.json
```
