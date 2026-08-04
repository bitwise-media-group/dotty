// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package privdot manages the encrypted private dotfiles repository: a
// second repository, kept separate from the shareable one, whose per-profile
// home trees hold the PII a public repo must never carry — git identities,
// ssh host configuration, known_hosts — as age ciphertext.
//
// The repository is marked by scaffold.PrivateMarker and mirrors the public
// layout at profiles/<name>/home, with each secret stored as <path>.age.
// Every profile carries its own age/recipients.txt and identity stubs, so a
// profile's files are encrypted only to that profile's keys. Plaintext never
// enters the repository: decryption lands under the dotty data directory
// (0700), from where the linker routes $HOME symlinks through an
// active-profile indirection.
//
// The crypto layer shells out to the age CLI and treats recipient and
// identity files as opaque, so any age plugin works; only enrollment knows
// the hardware backend (age-plugin-yubikey).
package privdot
