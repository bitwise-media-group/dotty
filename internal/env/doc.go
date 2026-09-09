// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

// Package env is the legacy credential store retained solely for
// `dotty env migrate`. Credentials were grouped into namespaces, each a single
// keychain item under the service name "dotty:<namespace>" holding a JSON
// object of key to value, and .env.dotty templates referenced them as
// "{{ dotty://<namespace>/<key> }}". Secrets now live in fnox; this package
// only reads (and, once migrated, deletes) what the old verbs stored, and
// scans the old templates so their lines can be carried into fnox.toml.
//
// The keychain access is platform-specific: the Keychain interface is backed
// by the macOS security(1) CLI in keychain_darwin.go, with a build-tagged stub
// for other platforms. The Store model, validation, and template scanning are
// portable.
package env
