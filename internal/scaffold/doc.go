// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package scaffold renders the dotfiles template embedded in the dotty
// binary into a net-new dotfiles repository. A manifest of components maps
// the init wizard's answers onto template paths; selected files are copied
// byte-for-byte except for a small allowlist rendered through text/template.
// The wizard's answers persist in each profile's profile.json inside the
// generated repository, so a re-run is an idempotent re-render rather than
// an interrogation.
package scaffold
