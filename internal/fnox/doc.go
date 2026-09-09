// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package fnox is a thin client over the fnox CLI (github.com/jdx/fnox), the
// secret manager dotty delegates credentials to. It builds argv for the few
// verbs dotty drives — set, remove, list, provider list, profiles — and
// bootstraps the provider pair dotty relies on: an age provider whose
// software identity is parked in the OS keychain through fnox's own keychain
// provider, so decrypting never prompts, with every enrolled YubiKey as an
// extra recipient for recovery.
//
// Values only ever reach fnox on stdin, never in argv, so they stay out of
// the process list. Everything else — profiles, the exec-only env mode, the
// shell hook — is fnox's own; see the credentials guide in docs/.
package fnox
