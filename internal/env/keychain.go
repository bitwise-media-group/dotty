// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"context"
	"errors"
)

// Keychain is the platform store behind the env command: one opaque item per
// namespace, addressed by name. The concrete implementation is selected at
// build time (see the GOOS-tagged keychain_*.go files) and constructed with
// NewKeychain.
type Keychain interface {
	// Read returns the stored bytes for namespace, or ErrNotFound when the
	// namespace has no item yet.
	Read(ctx context.Context, namespace string) ([]byte, error)
	// Write creates or replaces the item for namespace.
	Write(ctx context.Context, namespace string, value []byte) error
	// Delete removes the item for namespace, returning ErrNotFound when absent.
	Delete(ctx context.Context, namespace string) error
}

// CommandRunner is the slice of *cli.ExecRunner the keychain backends need:
// capture a program's stdout. Kept here (not in the darwin file) so every
// NewKeychain variant shares one signature and tests can fake it.
type CommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

var (
	// ErrNotFound reports a namespace with no keychain item behind it.
	ErrNotFound = errors.New("namespace not found in keychain")
	// ErrUnsupported reports a platform with no keychain backend wired up.
	ErrUnsupported = errors.New("dotty env is only supported on macOS (Linux keychain support is planned)")
)

// serviceName is the keychain service that isolates a namespace's credentials.
// The "dotty:" prefix keeps these items from colliding with anything else in
// the keychain.
func serviceName(namespace string) string {
	return "dotty:" + namespace
}
