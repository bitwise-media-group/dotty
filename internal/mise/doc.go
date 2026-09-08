// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package mise drives mise to keep a profile's packages — and the machine —
// reproducible. A profile carries a whole mise global-config directory
// (config.toml, conf.d/ fragments, mise.lock) that dotty links to
// ~/.config/mise; every invocation here retargets mise at that directory
// with MISE_CONFIG_DIR so the active profile and an explicitly named one go
// through one code path. Two kinds of entry live there: [tools] are locked
// per platform in mise.lock, [bootstrap.packages] are poured into the
// Homebrew prefix (or the OS package manager) at their latest version.
package mise
