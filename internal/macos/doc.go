// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package macos names the curated macOS defaults groups the init wizard
// offers — mise applies each through its fragment in the profile — and
// enforces PIV smart-card login, which needs sudo and so stays in dotty.
// Commands shell out through a Runner, so the package builds and tests
// everywhere — callers gate on runtime.GOOS before invoking it on a machine.
package macos
