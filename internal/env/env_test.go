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

func newFakeKeychain(items map[string]string) *fakeKeychain {
	f := &fakeKeychain{items: map[string][]byte{}}
	for ns, v := range items {
		f.items[ns] = []byte(v)
	}
	return f
}

func (f *fakeKeychain) Read(_ context.Context, ns string) ([]byte, error) {
	v, ok := f.items[ns]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), v...), nil
}

func (f *fakeKeychain) Delete(_ context.Context, ns string) error {
	if _, ok := f.items[ns]; !ok {
		return ErrNotFound
	}
	delete(f.items, ns)
	return nil
}

func (f *fakeKeychain) List(context.Context) ([]string, error) {
	names := make([]string, 0, len(f.items))
	for ns := range f.items {
		names = append(names, ns)
	}
	return names, nil
}

func TestStoreGet(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain(map[string]string{"aws": `{"AWS_KEY":"secret","MULTI":"line1\nline2 \"q\""}`}))

	got, err := s.Get(ctx, "aws", "AWS_KEY")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "secret" {
		t.Errorf("Get = %q, want %q", got, "secret")
	}
	// Values with newlines and quotes survive the JSON decode.
	if got, err := s.Get(ctx, "aws", "MULTI"); err != nil || got != "line1\nline2 \"q\"" {
		t.Errorf("Get MULTI = %q (err %v)", got, err)
	}

	if _, err := s.Get(ctx, "aws", "MISSING"); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get missing key err = %v, want ErrKeyNotFound", err)
	}
	if _, err := s.Get(ctx, "nope", "AWS_KEY"); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get missing namespace err = %v, want ErrKeyNotFound", err)
	}
}

func TestStoreGetMalformedItem(t *testing.T) {
	s := NewStore(newFakeKeychain(map[string]string{"bad": "not json"}))
	if _, err := s.Get(context.Background(), "bad", "K"); err == nil {
		t.Fatal("Get on a malformed item = nil error")
	}
}

func TestStoreKeysSorted(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain(map[string]string{"ns": `{"ZED":"v","ALPHA":"v","MIKE":"v"}`}))
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

func TestStoreDeleteNamespace(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newFakeKeychain(map[string]string{"ns": `{"A":"1","B":"2"}`}))
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
	s := NewStore(newFakeKeychain(map[string]string{"ns": `{"A":"1","B":"2"}`}))
	all, err := s.All(context.Background(), "ns")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(all, map[string]string{"A": "1", "B": "2"}) {
		t.Errorf("All = %v", all)
	}
}

func TestStoreNamespacesSorted(t *testing.T) {
	s := NewStore(newFakeKeychain(map[string]string{"zeta": "{}", "aws": "{}", "ci": "{}"}))
	got, err := s.Namespaces(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"aws", "ci", "zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Namespaces = %v, want %v", got, want)
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
