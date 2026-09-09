// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package envmigrate carries credentials out of the legacy dotty env store
// into fnox: every keychain namespace becomes a fnox profile in the global
// config (the "default" namespace lands in the top-level secrets), and every
// .env.dotty template becomes a fnox.toml beside it, literals as defaults
// and references as encrypted values. Each entry is warned about and counted
// on failure rather than aborting the run, re-runs skip what is already
// present, and the old artefacts are only removed when asked (--purge) and
// only once their namespace migrated cleanly.
package envmigrate
