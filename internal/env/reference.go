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
// "KEY" form, which the caller resolves against the default namespace.
type Reference struct {
	Namespace string
	Key       string
}

// ParseRef parses a reference body — the text inside a "{{ ... }}". It
// accepts "dotty://<namespace>/<key>" and a plain "<key>"; surrounding
// whitespace is ignored and the key must be a valid environment variable
// name.
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

// Refs parses every "{{ ... }}" reference in value, in order. whole reports
// that value is exactly one reference with nothing but whitespace around it —
// the only shape that maps onto a single fnox secret; anything else with a
// reference in it needs fnox's own interpolation. A reference that fails to
// parse, or an unterminated one, is an error: a reference is never silently
// treated as text.
func Refs(value string) (refs []Reference, whole bool, err error) {
	rest := value
	for {
		open := strings.Index(rest, "{{")
		if open < 0 {
			break
		}
		after := rest[open+2:]
		end := strings.Index(after, "}}")
		if end < 0 {
			return nil, false, fmt.Errorf("unterminated reference %q", rest[open:])
		}
		ref, err := ParseRef(after[:end])
		if err != nil {
			return nil, false, err
		}
		refs = append(refs, ref)
		rest = after[end+2:]
	}
	if len(refs) != 1 {
		return refs, false, nil
	}
	trimmed := strings.TrimSpace(value)
	whole = strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") &&
		strings.Count(trimmed, "{{") == 1
	return refs, whole, nil
}
