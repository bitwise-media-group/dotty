// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package envmigrate carries credentials out of the legacy dotty env store
// into fnox: every keychain namespace becomes a fnox profile in the global
// config (the "default" namespace lands in the top-level secrets), and every
// .env.dotty template is split: its plain KEY=value lines become [env]
// variables in the mise.toml beside it, and its references encrypted values
// in a fnox.toml beside it. Each entry is warned about and counted
// on failure rather than aborting the run, re-runs skip what is already
// present, and the old artefacts are only removed when asked (--purge) and
// only once their namespace migrated cleanly.
package envmigrate
