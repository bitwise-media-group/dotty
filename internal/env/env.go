// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

var (
	// ErrKeyNotFound reports a key with no value in its namespace.
	ErrKeyNotFound = errors.New("key not found")
	// ErrInvalidKey reports a key that is not a valid environment variable name.
	ErrInvalidKey = errors.New("invalid key")
	// ErrInvalidNamespace reports a namespace name that cannot be used.
	ErrInvalidNamespace = errors.New("invalid namespace")
)

// Store reads and writes credentials through a Keychain. Each namespace is one
// keychain item holding a JSON object of key to value; every mutation is a
// read-modify-write of that whole object.
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

// save writes values back, deleting the namespace item entirely when it is
// empty so no stray keychain entry lingers.
func (s *Store) save(ctx context.Context, namespace string, values map[string]string) error {
	if len(values) == 0 {
		return s.DeleteNamespace(ctx, namespace)
	}
	data, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("encode namespace %q: %w", namespace, err)
	}
	return s.kc.Write(ctx, namespace, data)
}

// Set stores key=value in namespace, creating the namespace if needed.
func (s *Store) Set(ctx context.Context, namespace, key, value string) error {
	if err := ValidateNamespace(namespace); err != nil {
		return err
	}
	if err := ValidateKey(key); err != nil {
		return err
	}
	values, err := s.load(ctx, namespace)
	if err != nil {
		return err
	}
	values[key] = value
	return s.save(ctx, namespace, values)
}

// SetAll stores every key=value in values into namespace as a single
// read-modify-write, creating the namespace if needed and overwriting any keys
// that already exist. Every key is validated before anything is written, so a
// bad key fails the whole batch rather than leaving it half-applied. An empty
// map is a no-op.
func (s *Store) SetAll(ctx context.Context, namespace string, values map[string]string) error {
	if err := ValidateNamespace(namespace); err != nil {
		return err
	}
	for key := range values {
		if err := ValidateKey(key); err != nil {
			return err
		}
	}
	if len(values) == 0 {
		return nil
	}
	current, err := s.load(ctx, namespace)
	if err != nil {
		return err
	}
	for key, value := range values {
		current[key] = value
	}
	return s.save(ctx, namespace, current)
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

// Delete removes key from namespace, reporting whether it was present. The
// namespace item is removed once its last key is gone.
func (s *Store) Delete(ctx context.Context, namespace, key string) (bool, error) {
	values, err := s.load(ctx, namespace)
	if err != nil {
		return false, err
	}
	if _, ok := values[key]; !ok {
		return false, nil
	}
	delete(values, key)
	if err := s.save(ctx, namespace, values); err != nil {
		return false, err
	}
	return true, nil
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
	sort.Strings(keys)
	return keys, nil
}

// All returns every key/value in namespace, empty when it does not exist.
func (s *Store) All(ctx context.Context, namespace string) (map[string]string, error) {
	return s.load(ctx, namespace)
}

// Resolver returns a lookup function for single credentials that caches each
// namespace's contents, so a template or process drawing many references reads
// the keychain once per namespace. A reference with an empty namespace (the
// bare {{ KEY }} form) falls back to fallback.
func (s *Store) Resolver(ctx context.Context, fallback string) func(namespace, key string) (string, error) {
	cache := map[string]map[string]string{}
	return func(namespace, key string) (string, error) {
		if namespace == "" {
			namespace = fallback
		}
		values, ok := cache[namespace]
		if !ok {
			var err error
			if values, err = s.load(ctx, namespace); err != nil {
				return "", err
			}
			cache[namespace] = values
		}
		value, found := values[key]
		if !found {
			return "", fmt.Errorf("%q in namespace %q: %w", key, namespace, ErrKeyNotFound)
		}
		return value, nil
	}
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
