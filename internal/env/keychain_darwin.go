// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

//go:build darwin

package env

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

// errSecItemNotFound is the status security(1) exits with when a queried item
// is absent (the errSecItemNotFound OSStatus, surfaced as 44 by the CLI).
const errSecItemNotFound = 44

// securityKeychain stores credentials in the macOS login keychain by shelling
// out to /usr/bin/security, one generic-password item per namespace. This
// mirrors how the rest of dotty drives external tools (ykman, ssh-keygen)
// rather than linking a CGO keychain library, so cross-compiled builds stay
// CGO-free.
//
// Write passes the JSON value via the -w argument, which is briefly visible to
// other processes of the same user via ps(1) — an accepted limitation of the
// security CLI. A future hardening could feed the value over the interactive
// prompt instead.
type securityKeychain struct {
	runner CommandRunner
}

// NewKeychain returns the macOS keychain backend.
func NewKeychain(runner CommandRunner) Keychain {
	return &securityKeychain{runner: runner}
}

func (k *securityKeychain) Read(ctx context.Context, namespace string) ([]byte, error) {
	out, err := k.runner.Output(ctx, "security",
		"find-generic-password", "-s", serviceName(namespace), "-a", namespace, "-w")
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// `security -w` prints the value followed by a single trailing newline; the
	// stored value (a one-line JSON object) never contains one of its own.
	return bytes.TrimSuffix(out, []byte("\n")), nil
}

func (k *securityKeychain) Write(ctx context.Context, namespace string, value []byte) error {
	_, err := k.runner.Output(ctx, "security",
		"add-generic-password", "-U",
		"-s", serviceName(namespace), "-a", namespace,
		"-D", "dotty env", "-w", string(value))
	return err
}

func (k *securityKeychain) Delete(ctx context.Context, namespace string) error {
	_, err := k.runner.Output(ctx, "security",
		"delete-generic-password", "-s", serviceName(namespace), "-a", namespace)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// isNotFound reports whether err is security(1) signalling a missing item.
func isNotFound(err error) bool {
	var ee *exec.ExitError
	return errors.As(err, &ee) && ee.ExitCode() == errSecItemNotFound
}

// serviceName is the keychain service that isolates a namespace's credentials.
// The "dotty:" prefix keeps these items from colliding with anything else in
// the keychain.
func serviceName(namespace string) string {
	return "dotty:" + namespace
}
