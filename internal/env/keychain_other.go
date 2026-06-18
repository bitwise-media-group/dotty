// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

//go:build !darwin

package env

import "context"

// NewKeychain returns a backend that reports ErrUnsupported for every
// operation. Only macOS has a keychain wired up so far; a Linux backend (e.g.
// secret-tool/libsecret) would live in a keychain_linux.go alongside this stub.
func NewKeychain(_ CommandRunner) Keychain {
	return unsupportedKeychain{}
}

type unsupportedKeychain struct{}

func (unsupportedKeychain) Read(context.Context, string) ([]byte, error) { return nil, ErrUnsupported }
func (unsupportedKeychain) Write(context.Context, string, []byte) error  { return ErrUnsupported }
func (unsupportedKeychain) Delete(context.Context, string) error         { return ErrUnsupported }
