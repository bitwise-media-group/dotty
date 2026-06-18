// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantNS  string
		wantKey string
		wantErr bool
	}{
		{name: "bare key", body: "TOKEN", wantKey: "TOKEN"},
		{name: "bare key padded", body: "  TOKEN  ", wantKey: "TOKEN"},
		{name: "full ref", body: "dotty://aws/AWS_KEY", wantNS: "aws", wantKey: "AWS_KEY"},
		{name: "full ref padded", body: "  dotty://aws/AWS_KEY ", wantNS: "aws", wantKey: "AWS_KEY"},
		{name: "empty", body: "", wantErr: true},
		{name: "bad key", body: "1BAD", wantErr: true},
		{name: "ref missing key", body: "dotty://aws/", wantErr: true},
		{name: "ref missing namespace", body: "dotty:///KEY", wantErr: true},
		{name: "ref no slash", body: "dotty://aws", wantErr: true},
		{name: "ref bad namespace", body: "dotty://a:b/KEY", wantErr: true},
		{name: "ref bad key", body: "dotty://aws/1BAD", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseRef(tt.body)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseRef(%q) = %+v, want error", tt.body, ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRef(%q): %v", tt.body, err)
			}
			if ref.Namespace != tt.wantNS || ref.Key != tt.wantKey {
				t.Errorf("ParseRef(%q) = {ns:%q key:%q}, want {ns:%q key:%q}",
					tt.body, ref.Namespace, ref.Key, tt.wantNS, tt.wantKey)
			}
		})
	}
}

// constResolve resolves any reference to "<ns>:<key>" so injection output is
// easy to assert; an empty namespace stands in for the bare form.
func constResolve(ns, key string) (string, error) {
	return ns + ":" + key, nil
}

func TestInject(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    string
		wantErr bool
	}{
		{name: "no refs", src: "plain text", want: "plain text"},
		{name: "empty", src: "", want: ""},
		{name: "single full ref", src: "a={{ dotty://aws/KEY }}", want: "a=aws:KEY"},
		{name: "bare ref", src: "a={{ KEY }}", want: "a=:KEY"},
		{name: "two refs", src: "{{ KEY }}-{{ dotty://ci/T }}", want: ":KEY-ci:T"},
		{name: "no inner spaces", src: "{{dotty://aws/KEY}}", want: "aws:KEY"},
		{name: "surrounding text", src: "before {{ KEY }} after", want: "before :KEY after"},
		{name: "unterminated", src: "x={{ KEY", wantErr: true},
		{name: "malformed ref", src: "x={{ 1BAD }}", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Inject(tt.src, constResolve)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Inject(%q) = %q, want error", tt.src, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Inject(%q): %v", tt.src, err)
			}
			if got != tt.want {
				t.Errorf("Inject(%q) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestInjectResolverError(t *testing.T) {
	boom := fmt.Errorf("kaboom")
	_, err := Inject("a={{ MISSING }}", func(string, string) (string, error) { return "", boom })
	if err == nil {
		t.Fatal("Inject with failing resolver returned nil error")
	}
}

func FuzzParseRef(f *testing.F) {
	for _, seed := range []string{"TOKEN", "dotty://aws/KEY", "", "dotty://", "1BAD", "  x  ", "dotty://a/b/c"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		ref, err := ParseRef(body) // must never panic
		if err == nil {
			if ref.Key == "" {
				t.Errorf("ParseRef(%q) returned ok with empty key", body)
			}
			if err := ValidateKey(ref.Key); err != nil {
				t.Errorf("ParseRef(%q) accepted invalid key %q", body, ref.Key)
			}
			if ref.Namespace != "" {
				if err := ValidateNamespace(ref.Namespace); err != nil {
					t.Errorf("ParseRef(%q) accepted invalid namespace %q", body, ref.Namespace)
				}
			}
		}
	})
}

func FuzzInject(f *testing.F) {
	for _, seed := range []string{"", "plain", "{{ KEY }}", "{{dotty://a/B}}", "x={{ KEY", "{{}}", "{{ {{ KEY }} }}"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		// A resolver that always succeeds: a successful Inject must leave no
		// reference opener behind, and must never panic.
		out, err := Inject(src, func(ns, key string) (string, error) { return "v", nil })
		if err == nil && strings.Contains(out, "{{") {
			t.Errorf("Inject(%q) = %q still contains %q", src, out, "{{")
		}
	})
}
