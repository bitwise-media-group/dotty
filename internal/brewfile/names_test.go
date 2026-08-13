// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package brewfile

import (
	"strings"
	"testing"
)

func TestCanonicalName(t *testing.T) {
	tests := []struct {
		kind Kind
		in   string
		want string
	}{
		{kind: KindFormula, in: "JQ", want: "jq"},
		{kind: KindFormula, in: "homebrew/homebrew/Foo", want: "foo"},
		{kind: KindFormula, in: "User/homebrew-tap/Pkg", want: "user/tap/pkg"},
		{kind: KindFormula, in: "user/tap/pkg", want: "user/tap/pkg"},
		{kind: KindFormula, in: "a/b/c/d", want: "a/b/c/d"}, // not a tap-qualified shape
		{kind: KindCask, in: "Ghostty", want: "ghostty"},
		{kind: KindCask, in: "acme/tap/Widget", want: "widget"},
		{kind: KindCask, in: "Acme/homebrew-tap/Widget", want: "widget"},
		{kind: KindTap, in: "FluxCD/homebrew-tap", want: "fluxcd/tap"},
		{kind: KindTap, in: "fluxcd/tap", want: "fluxcd/tap"},
		{kind: KindTap, in: "not-a-tap", want: "not-a-tap"},
		{kind: KindNPM, in: "Left-Pad", want: "Left-Pad"}, // extension kinds are verbatim
		{kind: KindVSCode, in: "GitHub.Copilot", want: "GitHub.Copilot"},
	}
	for _, tt := range tests {
		if got := canonicalName(tt.kind, tt.in); got != tt.want {
			t.Errorf("canonicalName(%s, %q) = %q, want %q", tt.kind, tt.in, got, tt.want)
		}
	}
}

func TestTrustStoreName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "jq", want: "jq"},
		{in: "Acme/homebrew-tap/Widget", want: "acme/tap/widget"},
		{in: "FluxCD/homebrew-tap", want: "fluxcd/tap"},
		{in: "acme/tap/widget", want: "acme/tap/widget"}, // casks keep full qualification
	}
	for _, tt := range tests {
		if got := trustStoreName(tt.in); got != tt.want {
			t.Errorf("trustStoreName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLineKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: `brew "jq"`, want: "brew jq"},
		{in: `brew "jq", trusted: true`, want: "brew jq"},            // options ignored
		{in: `  brew "JQ"  `, want: "brew jq"},                       // whitespace and casing
		{in: `brew "homebrew/homebrew/foo"`, want: "brew foo"},       // formula canonicalisation
		{in: `cask "Acme/homebrew-tap/Widget"`, want: "cask widget"}, // casks reduce to short token
		{in: `tap "FluxCD/homebrew-tap"`, want: "tap fluxcd/tap"},
		{in: `vscode "GitHub.Copilot"`, want: "vscode GitHub.Copilot"}, // extension kinds verbatim
		{in: `mas "Xcode", id: 497799835`, want: "mas Xcode"},
		{in: "# a comment", want: ""},
		{in: "", want: ""},
		{in: "   ", want: ""},
		{in: `install "jq"`, want: ""},       // unknown DSL word
		{in: "brew jq", want: ""},            // unquoted name
		{in: `brew "unterminated`, want: ""}, // unclosed quote
		{in: `brew ""`, want: ""},            // empty name
	}
	for _, tt := range tests {
		if got := LineKey(tt.in); got != tt.want {
			t.Errorf("LineKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDslWord(t *testing.T) {
	for kind, want := range map[Kind]string{KindFormula: "brew", KindCask: "cask", KindTap: "tap", KindNPM: "npm"} {
		if got := dslWord(kind); got != want {
			t.Errorf("dslWord(%s) = %q, want %q", kind, got, want)
		}
	}
}

func FuzzCanonicalName(f *testing.F) {
	f.Add("jq")
	f.Add("Acme/homebrew-tap/Widget")
	f.Add("homebrew/homebrew/foo")
	f.Add("a/b/c/d")
	f.Add("///")
	f.Add("")
	f.Fuzz(func(t *testing.T, name string) {
		// Arbitrary names must never panic, and every trustable kind's
		// sanitiser downcases its result.
		for _, kind := range []Kind{KindFormula, KindCask, KindTap} {
			got := canonicalName(kind, name)
			if got != strings.ToLower(got) {
				t.Errorf("canonicalName(%s, %q) = %q, not lowercase", kind, name, got)
			}
		}
		_ = trustStoreName(name)
	})
}
