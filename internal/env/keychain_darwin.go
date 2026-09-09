// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

//go:build darwin

package env

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"slices"
)

// errSecItemNotFound is the status security(1) exits with when a queried item
// is absent (the errSecItemNotFound OSStatus, surfaced as 44 by the CLI).
const errSecItemNotFound = 44

// servicePrefix is the keychain service prefix that isolated the legacy
// namespaces from everything else in the keychain.
const servicePrefix = "dotty:"

// serviceAttrRe matches the service attribute line of a dump-keychain
// record, capturing the namespace behind the dotty prefix.
var serviceAttrRe = regexp.MustCompile(`"svce"<blob>="` + regexp.QuoteMeta(servicePrefix) + `([^"]+)"`)

// securityKeychain reads the credentials the legacy verbs stored in the macOS
// login keychain by shelling out to /usr/bin/security, one generic-password
// item per namespace. This mirrors how the rest of dotty drives external
// tools (ykman, ssh-keygen) rather than linking a CGO keychain library, so
// cross-compiled builds stay CGO-free.
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

// List enumerates the namespaces by dumping the keychain's item attributes —
// without -d, so no secret data is read or prompted for — and collecting the
// services carrying the dotty prefix.
func (k *securityKeychain) List(ctx context.Context) ([]string, error) {
	out, err := k.runner.Output(ctx, "security", "dump-keychain")
	if err != nil {
		return nil, err
	}
	var namespaces []string
	for _, m := range serviceAttrRe.FindAllStringSubmatch(string(out), -1) {
		if ns := m[1]; !slices.Contains(namespaces, ns) {
			namespaces = append(namespaces, ns)
		}
	}
	return namespaces, nil
}

// isNotFound reports whether err is security(1) signalling a missing item.
func isNotFound(err error) bool {
	var ee *exec.ExitError
	return errors.As(err, &ee) && ee.ExitCode() == errSecItemNotFound
}

// serviceName is the keychain service that isolates a namespace's credentials.
func serviceName(namespace string) string {
	return servicePrefix + namespace
}
