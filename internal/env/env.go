// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

var (
	// ErrKeyNotFound reports a key with no value in its namespace.
	ErrKeyNotFound = errors.New("key not found")
	// ErrInvalidKey reports a key that is not a valid environment variable name.
	ErrInvalidKey = errors.New("invalid key")
	// ErrInvalidNamespace reports a namespace name that cannot be used.
	ErrInvalidNamespace = errors.New("invalid namespace")
)

// Store reads the credentials the legacy verbs kept through a Keychain. Each
// namespace is one keychain item holding a JSON object of key to value.
type Store struct {
	kc Keychain
}

// NewStore returns a Store backed by kc.
func NewStore(kc Keychain) *Store {
	return &Store{kc: kc}
}

// load returns the namespace's map, or an empty map when the namespace has no
// item yet.
func (s *Store) load(ctx context.Context, namespace string) (map[string]string, error) {
	data, err := s.kc.Read(ctx, namespace)
	if errors.Is(err, ErrNotFound) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	if len(data) == 0 {
		return values, nil
	}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("decode namespace %q: %w", namespace, err)
	}
	return values, nil
}

// Get returns the value of key in namespace, or ErrKeyNotFound.
func (s *Store) Get(ctx context.Context, namespace, key string) (string, error) {
	values, err := s.load(ctx, namespace)
	if err != nil {
		return "", err
	}
	value, ok := values[key]
	if !ok {
		return "", fmt.Errorf("%q in namespace %q: %w", key, namespace, ErrKeyNotFound)
	}
	return value, nil
}

// DeleteNamespace removes a namespace and all of its keys. A namespace that
// does not exist is not an error.
func (s *Store) DeleteNamespace(ctx context.Context, namespace string) error {
	if err := s.kc.Delete(ctx, namespace); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}

// Keys returns the sorted key names in namespace, empty when the namespace does
// not exist.
func (s *Store) Keys(ctx context.Context, namespace string) ([]string, error) {
	values, err := s.load(ctx, namespace)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys, nil
}

// All returns every key/value in namespace, empty when it does not exist.
func (s *Store) All(ctx context.Context, namespace string) (map[string]string, error) {
	return s.load(ctx, namespace)
}

// Namespaces returns every namespace with a keychain item, sorted.
func (s *Store) Namespaces(ctx context.Context) ([]string, error) {
	namespaces, err := s.kc.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.Sort(namespaces)
	return namespaces, nil
}

// ValidateKey reports whether key is a usable environment variable name: a
// leading letter or underscore followed by letters, digits, or underscores.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty", ErrInvalidKey)
	}
	for i, r := range key {
		switch {
		case r == '_', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return fmt.Errorf("%w: %q (use letters, digits, and underscores; no leading digit)", ErrInvalidKey, key)
		}
	}
	return nil
}

// ValidateNamespace reports whether namespace is usable in the keychain service
// name: non-empty and limited to letters, digits, '.', '-', and '_'. A colon is
// rejected because it delimits the "dotty:" service prefix.
func ValidateNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("%w: empty", ErrInvalidNamespace)
	}
	for _, r := range namespace {
		switch {
		case r == '.', r == '-', r == '_':
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		default:
			return fmt.Errorf("%w: %q (use letters, digits, '.', '-', '_')", ErrInvalidNamespace, namespace)
		}
	}
	return nil
}
