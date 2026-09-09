// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"context"
	"errors"
)

// Keychain is the platform store the legacy env verbs wrote to: one opaque
// item per namespace, addressed by name. The concrete implementation is
// selected at build time (see the GOOS-tagged keychain_*.go files) and
// constructed with NewKeychain. Only what migration needs remains: read,
// enumerate, delete.
type Keychain interface {
	// Read returns the stored bytes for namespace, or ErrNotFound when the
	// namespace has no item yet.
	Read(ctx context.Context, namespace string) ([]byte, error)
	// Delete removes the item for namespace, returning ErrNotFound when absent.
	Delete(ctx context.Context, namespace string) error
	// List returns every namespace with an item, unsorted.
	List(ctx context.Context) ([]string, error)
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
	ErrUnsupported = errors.New("the legacy dotty env keychain store only exists on macOS")
)
