// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// fakeKeychain is an in-memory Keychain so Store behavior is testable without
// touching the real OS keychain.
type fakeKeychain struct {
	items map[string][]byte
}

func newFakeKeychain() *fakeKeychain { return &fakeKeychain{items: map[string][]byte{}} }

func (f *fakeKeychain) Read(_ context.Context, ns string) ([]byte, error) {
	v, ok := f.items[ns]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), v...), nil
}

func (f *fakeKeychain) Write(_ context.Context, ns string, value []byte) error {
	f.items[ns] = append([]byte(nil), value...)
	return nil
}

func (f *fakeKeychain) Delete(_ context.Context, ns string) error {
	if _, ok := f.items[ns]; !ok {
		return ErrNotFound
	}
	delete(f.items, ns)
	return nil
}

func TestStoreSetGet(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain())

	if err := s.Set(ctx, "aws", "AWS_KEY", "secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := s.Get(ctx, "aws", "AWS_KEY")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "secret" {
		t.Errorf("Get = %q, want %q", got, "secret")
	}

	if _, err := s.Get(ctx, "aws", "MISSING"); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get missing key err = %v, want ErrKeyNotFound", err)
	}
	if _, err := s.Get(ctx, "nope", "AWS_KEY"); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get missing namespace err = %v, want ErrKeyNotFound", err)
	}
}

func TestStoreOverwriteAndValueShapes(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain())

	if err := s.Set(ctx, "ns", "K", "first"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, "ns", "K", "second"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "ns", "K")
	if err != nil {
		t.Fatal(err)
	}
	if got != "second" {
		t.Errorf("overwrite Get = %q, want %q", got, "second")
	}

	// Values with newlines and quotes survive the JSON round-trip.
	tricky := "line1\nline2 \"quoted\" \t end"
	if err := s.Set(ctx, "ns", "MULTI", tricky); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(ctx, "ns", "MULTI"); err != nil || got != tricky {
		t.Errorf("tricky value round-trip = %q (err %v), want %q", got, err, tricky)
	}
}

func TestStoreKeysSorted(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain())
	for _, k := range []string{"ZED", "ALPHA", "MIKE"} {
		if err := s.Set(ctx, "ns", k, "v"); err != nil {
			t.Fatal(err)
		}
	}
	keys, err := s.Keys(ctx, "ns")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ALPHA", "MIKE", "ZED"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("Keys = %v, want %v", keys, want)
	}

	empty, err := s.Keys(ctx, "absent")
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Errorf("Keys(absent) = %v, want empty", empty)
	}
}

func TestStoreDeleteLastRemovesNamespaceItem(t *testing.T) {
	ctx := context.Background()
	kc := newFakeKeychain()
	s := NewStore(kc)

	if err := s.Set(ctx, "ns", "ONLY", "v"); err != nil {
		t.Fatal(err)
	}
	if _, ok := kc.items["ns"]; !ok {
		t.Fatal("namespace item not created")
	}

	found, err := s.Delete(ctx, "ns", "ONLY")
	if err != nil || !found {
		t.Fatalf("Delete = (%v, %v), want (true, nil)", found, err)
	}
	if _, ok := kc.items["ns"]; ok {
		t.Error("namespace item should be removed once its last key is deleted")
	}
}

func TestStoreDeleteMissingKey(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain())
	if err := s.Set(ctx, "ns", "A", "v"); err != nil {
		t.Fatal(err)
	}
	found, err := s.Delete(ctx, "ns", "MISSING")
	if err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
	if found {
		t.Error("Delete missing key reported found = true")
	}
}

func TestStoreDeleteNamespace(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain())
	for _, k := range []string{"A", "B"} {
		if err := s.Set(ctx, "ns", k, "v"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteNamespace(ctx, "ns"); err != nil {
		t.Fatalf("DeleteNamespace: %v", err)
	}
	if keys, _ := s.Keys(ctx, "ns"); len(keys) != 0 {
		t.Errorf("after DeleteNamespace, Keys = %v, want empty", keys)
	}
	// Deleting an absent namespace is not an error.
	if err := s.DeleteNamespace(ctx, "ns"); err != nil {
		t.Errorf("DeleteNamespace(absent) = %v, want nil", err)
	}
}

func TestStoreAll(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain())
	if err := s.Set(ctx, "ns", "A", "1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, "ns", "B", "2"); err != nil {
		t.Fatal(err)
	}
	all, err := s.All(ctx, "ns")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(all, map[string]string{"A": "1", "B": "2"}) {
		t.Errorf("All = %v", all)
	}
}

func TestStoreResolver(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain())
	if err := s.Set(ctx, "aws", "KEY", "aws-secret"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(ctx, "ci", "TOKEN", "ci-secret"); err != nil {
		t.Fatal(err)
	}
	resolve := s.Resolver(ctx, "aws")

	// Bare key (empty namespace) falls back to "aws".
	if v, err := resolve("", "KEY"); err != nil || v != "aws-secret" {
		t.Errorf("resolve fallback = %q (err %v), want aws-secret", v, err)
	}
	// Explicit namespace.
	if v, err := resolve("ci", "TOKEN"); err != nil || v != "ci-secret" {
		t.Errorf("resolve explicit = %q (err %v), want ci-secret", v, err)
	}
	// Unknown key errors.
	if _, err := resolve("aws", "NOPE"); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("resolve unknown err = %v, want ErrKeyNotFound", err)
	}
}

func TestStoreSetValidates(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain())
	if err := s.Set(ctx, "ns", "1BAD", "v"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("Set invalid key err = %v, want ErrInvalidKey", err)
	}
	if err := s.Set(ctx, "bad:ns", "OK", "v"); !errors.Is(err, ErrInvalidNamespace) {
		t.Errorf("Set invalid namespace err = %v, want ErrInvalidNamespace", err)
	}
}

func TestValidateKey(t *testing.T) {
	tests := []struct {
		key string
		ok  bool
	}{
		{"AWS_KEY", true},
		{"_private", true},
		{"a1", true},
		{"PATH123", true},
		{"", false},
		{"1LEADING", false},
		{"has space", false},
		{"has-dash", false},
		{"has=eq", false},
		{"dotty://x", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if tt.ok && err != nil {
				t.Errorf("ValidateKey(%q) = %v, want nil", tt.key, err)
			}
			if !tt.ok && err == nil {
				t.Errorf("ValidateKey(%q) = nil, want error", tt.key)
			}
		})
	}
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		ns string
		ok bool
	}{
		{"default", true},
		{"aws", true},
		{"team.staging", true},
		{"a-b_c.1", true},
		{"", false},
		{"has space", false},
		{"has:colon", false},
		{"slash/ns", false},
	}
	for _, tt := range tests {
		t.Run(tt.ns, func(t *testing.T) {
			err := ValidateNamespace(tt.ns)
			if tt.ok && err != nil {
				t.Errorf("ValidateNamespace(%q) = %v, want nil", tt.ns, err)
			}
			if !tt.ok && err == nil {
				t.Errorf("ValidateNamespace(%q) = nil, want error", tt.ns)
			}
		})
	}
}
