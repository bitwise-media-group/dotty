// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"fmt"
	"strings"
)

// refScheme prefixes a fully-qualified credential reference.
const refScheme = "dotty://"

// Reference is a parsed credential reference. Namespace is empty for the bare
// "KEY" form, which the caller resolves against the active namespace.
type Reference struct {
	Namespace string
	Key       string
}

// ParseRef parses a reference body — the text inside a "{{ ... }}" or a bare
// argument to env get. It accepts "dotty://<namespace>/<key>" and a plain
// "<key>"; surrounding whitespace is ignored and the key must be a valid
// environment variable name.
func ParseRef(body string) (Reference, error) {
	body = strings.TrimSpace(body)
	if rest, ok := strings.CutPrefix(body, refScheme); ok {
		ns, key, found := strings.Cut(rest, "/")
		if !found {
			return Reference{}, fmt.Errorf("malformed reference %q (want %s<namespace>/<key>)", body, refScheme)
		}
		if err := ValidateNamespace(ns); err != nil {
			return Reference{}, err
		}
		if err := ValidateKey(key); err != nil {
			return Reference{}, err
		}
		return Reference{Namespace: ns, Key: key}, nil
	}
	if err := ValidateKey(body); err != nil {
		return Reference{}, fmt.Errorf("malformed reference %q: %w", body, err)
	}
	return Reference{Key: body}, nil
}

// Inject replaces every "{{ ... }}" reference in src with the value returned by
// resolve, which receives the reference's namespace (empty for the bare form,
// for the resolver to default) and key. Text outside references is copied
// verbatim. A reference that fails to parse or resolve is a hard error — a
// reference is never silently blanked, matching `op inject`.
func Inject(src string, resolve func(namespace, key string) (string, error)) (string, error) {
	var b strings.Builder
	rest := src
	for {
		open := strings.Index(rest, "{{")
		if open < 0 {
			b.WriteString(rest)
			return b.String(), nil
		}
		b.WriteString(rest[:open])
		after := rest[open+2:]
		end := strings.Index(after, "}}")
		if end < 0 {
			return "", fmt.Errorf("unterminated reference %q", rest[open:])
		}
		ref, err := ParseRef(after[:end])
		if err != nil {
			return "", err
		}
		value, err := resolve(ref.Namespace, ref.Key)
		if err != nil {
			return "", err
		}
		b.WriteString(value)
		rest = after[end+2:]
	}
}
