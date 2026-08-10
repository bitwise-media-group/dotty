// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package brewfile

import (
	"context"
	"testing"
)

func TestNeedsTrust(t *testing.T) {
	tests := []struct {
		kind Kind
		name string
		want bool
	}{
		{kind: KindFormula, name: "jq", want: false},
		{kind: KindFormula, name: "fluxcd/tap", want: false}, // one slash: a plain tap name
		{kind: KindFormula, name: "acme/tap/widget", want: true},
		{kind: KindFormula, name: "a/b/c/d", want: true},
		{kind: KindFormula, name: "homebrew/homebrew/jq", want: false}, // official, always trusted
		{kind: KindFormula, name: "", want: false},
		{kind: KindCask, name: "ghostty", want: false},
		{kind: KindCask, name: "acme/tap/widget", want: true},
		{kind: KindTap, name: "fluxcd/tap", want: true},
		{kind: KindTap, name: "Homebrew/services", want: false}, // official, always trusted
		{kind: KindTap, name: "acme/tap/widget", want: false},   // not a tap name
		{kind: KindTap, name: "jq", want: false},
		{kind: KindNPM, name: "a/b/c", want: false}, // not trustable at all
	}
	for _, tt := range tests {
		if got := NeedsTrust(tt.kind, tt.name); got != tt.want {
			t.Errorf("NeedsTrust(%s, %q) = %v, want %v", tt.kind, tt.name, got, tt.want)
		}
	}
}

func TestIsTrusted(t *testing.T) {
	// Shape captured from a live `brew trust --json v1` on Homebrew 6.0.0.
	doc := `{
	  "taps": ["fluxcd/tap", "hashicorp/tap"],
	  "formulae": ["anomalyco/tap/opencode", "derailed/k9s/k9s"],
	  "casks": ["terraform-linters/tap/tflint"],
	  "commands": []
	}`
	tests := []struct {
		name    string
		kind    Kind
		target  string
		want    bool
		wantErr bool
	}{
		{name: "trusted formula", kind: KindFormula, target: "anomalyco/tap/opencode", want: true},
		{name: "unnormalised spelling of a trusted formula", kind: KindFormula,
			target: "Anomalyco/homebrew-tap/OpenCode", want: true},
		{name: "untrusted formula", kind: KindFormula, target: "acme/tap/widget", want: false},
		{name: "trusted cask", kind: KindCask, target: "terraform-linters/tap/tflint", want: true},
		{name: "trusted tap", kind: KindTap, target: "fluxcd/tap", want: true},
		{name: "cask name not valid as formula", kind: KindFormula, target: "terraform-linters/tap/tflint", want: false},
		{name: "untrustable kind errors", kind: KindNPM, target: "x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{outputs: [][]byte{[]byte(doc)}}
			got, err := IsTrusted(context.Background(), r, tt.kind, tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsTrusted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrustRejectsUntrustableKinds(t *testing.T) {
	r := &fakeRunner{}
	if err := Trust(context.Background(), r, KindGo, "x"); err == nil {
		t.Error("Trust(go) error = nil, want failure")
	}
	if len(r.calls) != 0 {
		t.Errorf("calls = %v, want none", r.calls)
	}
}

func TestUntrust(t *testing.T) {
	r := &fakeRunner{}
	if err := Untrust(context.Background(), r, KindFormula, "acme/tap/widget"); err != nil {
		t.Fatalf("Untrust() error: %v", err)
	}
	if want := "brew untrust --formula acme/tap/widget"; r.argv(0) != want {
		t.Errorf("argv = %q, want %q", r.argv(0), want)
	}
}

func TestUntrustRejectsUntrustableKinds(t *testing.T) {
	r := &fakeRunner{}
	if err := Untrust(context.Background(), r, KindGo, "x"); err == nil {
		t.Error("Untrust(go) error = nil, want failure")
	}
	if len(r.calls) != 0 {
		t.Errorf("calls = %v, want none", r.calls)
	}
}

func FuzzDecodeTrustList(f *testing.F) {
	f.Add(`{"taps":[],"formulae":[],"casks":[],"commands":[]}`)
	f.Add(`{"taps":["a/b"],"formulae":["a/b/c"]}`)
	f.Add(`not json at all`)
	f.Add(`[]`)
	f.Fuzz(func(t *testing.T, data string) {
		// External-tool output must never panic the decoder; errors are fine.
		_, _ = decodeTrustList([]byte(data))
	})
}
