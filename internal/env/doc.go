// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package env stores generic credentials in the operating system keychain and
// injects them into templates and processes, the way the 1Password CLI does but
// with no external service. Credentials are grouped into namespaces; each
// namespace is a single keychain item under the service name "dotty:<namespace>"
// holding a JSON object of key to value.
//
// The keychain access is platform-specific: the Keychain interface is backed by
// the macOS security(1) CLI in keychain_darwin.go, with a build-tagged stub for
// other platforms. Everything else here — the Store blob model, key/namespace
// validation, and the {{ dotty://... }} reference injection — is portable.
package env
