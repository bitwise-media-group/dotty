// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

//go:build !darwin

package env

import "context"

// NewKeychain returns a backend that reports ErrUnsupported for every
// operation. Only macOS ever had a keychain wired up, so on other platforms
// there is nothing to migrate.
func NewKeychain(_ CommandRunner) Keychain {
	return unsupportedKeychain{}
}

type unsupportedKeychain struct{}

func (unsupportedKeychain) Read(context.Context, string) ([]byte, error) { return nil, ErrUnsupported }
func (unsupportedKeychain) Delete(context.Context, string) error         { return ErrUnsupported }
func (unsupportedKeychain) List(context.Context) ([]string, error)       { return nil, ErrUnsupported }
