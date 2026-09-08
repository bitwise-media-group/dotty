// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package cli provides the cross-area plumbing every dotty command builds on:
// IO streams, an exec runner for the external tools dotty orchestrates (mise,
// ykman, fido2-token, ssh-keygen), XDG path resolution with dotty's
// public-config / private-data split, $EDITOR round-trips, and argv helpers
// for proxy commands.
package cli
