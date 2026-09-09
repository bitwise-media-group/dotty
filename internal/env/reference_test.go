// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"reflect"
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

func TestRefs(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantRefs  []Reference
		wantWhole bool
		wantErr   bool
	}{
		{name: "no refs", value: "plain text"},
		{name: "empty", value: ""},
		{name: "single full ref", value: "{{ dotty://aws/KEY }}", wantRefs: []Reference{{"aws", "KEY"}}, wantWhole: true},
		{name: "bare ref", value: "{{ KEY }}", wantRefs: []Reference{{"", "KEY"}}, wantWhole: true},
		{name: "no inner spaces", value: "{{dotty://aws/KEY}}", wantRefs: []Reference{{"aws", "KEY"}}, wantWhole: true},
		{name: "padded whole ref", value: "  {{ KEY }} ", wantRefs: []Reference{{"", "KEY"}}, wantWhole: true},
		{name: "two refs", value: "{{ KEY }}-{{ dotty://ci/T }}", wantRefs: []Reference{{"", "KEY"}, {"ci", "T"}}},
		{name: "surrounding text", value: "before {{ KEY }} after", wantRefs: []Reference{{"", "KEY"}}},
		{name: "unterminated", value: "x={{ KEY", wantErr: true},
		{name: "malformed ref", value: "x={{ 1BAD }}", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, whole, err := Refs(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Refs(%q) = %v, %v, want error", tt.value, refs, whole)
				}
				return
			}
			if err != nil {
				t.Fatalf("Refs(%q): %v", tt.value, err)
			}
			if !reflect.DeepEqual(refs, tt.wantRefs) || whole != tt.wantWhole {
				t.Errorf("Refs(%q) = %v, %v; want %v, %v", tt.value, refs, whole, tt.wantRefs, tt.wantWhole)
			}
		})
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
