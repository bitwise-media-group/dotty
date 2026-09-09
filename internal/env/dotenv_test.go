// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"strings"
	"testing"
)

func TestLines(t *testing.T) {
	ref := func(ns, key string) *Reference { return &Reference{Namespace: ns, Key: key} }
	tests := []struct {
		name string
		src  string
		want []Line
	}{
		{name: "literal passthrough", src: "LOG_LEVEL=debug", want: []Line{{N: 1, Key: "LOG_LEVEL", Value: "debug"}}},
		{name: "double quotes decoded", src: `MSG="line1\nline2"`, want: []Line{{N: 1, Key: "MSG", Value: "line1\nline2"}}},
		{name: "single quotes literal", src: `RAW='a\nb'`, want: []Line{{N: 1, Key: "RAW", Value: `a\nb`}}},
		{name: "inline comment stripped", src: "KEY=value # trailing note", want: []Line{{N: 1, Key: "KEY", Value: "value"}}},
		{name: "hash without leading space kept", src: "PASS=p#ss", want: []Line{{N: 1, Key: "PASS", Value: "p#ss"}}},
		{name: "export prefix and spacing", src: "export  AWS_KEY = secret",
			want: []Line{{N: 1, Key: "AWS_KEY", Value: "secret"}}},
		{name: "empty value is kept", src: "EMPTY=\nQUOTED_EMPTY=\"\"",
			want: []Line{{N: 1, Key: "EMPTY"}, {N: 2, Key: "QUOTED_EMPTY"}}},
		{name: "non-assignment lines skipped but numbered", src: "# c\nnot an assignment\n1BAD=x\nGOOD=ok",
			want: []Line{{N: 4, Key: "GOOD", Value: "ok"}}},
		{name: "crlf endings", src: "A=1\r\nB=2\r\n",
			want: []Line{{N: 1, Key: "A", Value: "1"}, {N: 2, Key: "B", Value: "2"}}},
		{name: "duplicate keys keep file order", src: "DUP=first\nDUP=second",
			want: []Line{{N: 1, Key: "DUP", Value: "first"}, {N: 2, Key: "DUP", Value: "second"}}},
		{name: "full reference", src: "API_KEY={{ dotty://prod/API_KEY }}",
			want: []Line{{N: 1, Key: "API_KEY", Value: "{{ dotty://prod/API_KEY }}", Ref: ref("prod", "API_KEY")}}},
		{name: "bare reference", src: "TOKEN={{ TOKEN }}",
			want: []Line{{N: 1, Key: "TOKEN", Value: "{{ TOKEN }}", Ref: ref("", "TOKEN")}}},
		{name: "quoted reference", src: `T="{{ dotty://a/B }}"`,
			want: []Line{{N: 1, Key: "T", Value: "{{ dotty://a/B }}", Ref: ref("a", "B")}}},
		{name: "blank input", src: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Lines(tt.src)
			if len(got) != len(tt.want) {
				t.Fatalf("Lines(%q) = %+v, want %+v", tt.src, got, tt.want)
			}
			for i := range got {
				g, w := got[i], tt.want[i]
				if g.Err != nil {
					t.Errorf("line %d: unexpected error %v", g.N, g.Err)
				}
				if g.N != w.N || g.Key != w.Key || g.Value != w.Value {
					t.Errorf("line %d = {%d %q %q}, want {%d %q %q}", i, g.N, g.Key, g.Value, w.N, w.Key, w.Value)
				}
				switch {
				case (g.Ref == nil) != (w.Ref == nil):
					t.Errorf("line %d ref = %v, want %v", i, g.Ref, w.Ref)
				case g.Ref != nil && *g.Ref != *w.Ref:
					t.Errorf("line %d ref = %+v, want %+v", i, *g.Ref, *w.Ref)
				}
			}
		})
	}
}

// TestLinesErrors pins the lines that cannot be carried into fnox as either
// a literal or a single secret; each is reported on its own line number with
// the other lines unaffected.
func TestLinesErrors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantN   int
		wantSub string
	}{
		{name: "unterminated quote", src: "OK=1\nOPEN=\"no close", wantN: 2, wantSub: "unterminated quote"},
		{name: "trailing junk after quote", src: `X="v"y`, wantN: 1, wantSub: "unterminated quote"},
		{name: "unterminated reference", src: "X={{ KEY", wantN: 1, wantSub: "unterminated reference"},
		{name: "malformed reference", src: "X={{ dotty://prod }}", wantN: 1, wantSub: "malformed reference"},
		{name: "mixed reference and text", src: "DSN=postgres://{{ dotty://prod/HOST }}/db",
			wantN: 1, wantSub: "interpolation"},
		{name: "two references", src: "X={{ A }}{{ B }}", wantN: 1, wantSub: "interpolation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bad []Line
			for _, l := range Lines(tt.src) {
				if l.Err != nil {
					bad = append(bad, l)
				}
			}
			if len(bad) != 1 {
				t.Fatalf("Lines(%q) errors = %+v, want exactly one", tt.src, bad)
			}
			if bad[0].N != tt.wantN || !strings.Contains(bad[0].Err.Error(), tt.wantSub) {
				t.Errorf("error = line %d %q, want line %d containing %q", bad[0].N, bad[0].Err, tt.wantN, tt.wantSub)
			}
		})
	}
}

func FuzzLines(f *testing.F) {
	for _, seed := range []string{
		"K=v", "export K = v # c", `Q="a\nb"`, "K={{ dotty://ns/K }}",
		"# comment", "", "K=", "1BAD=x", "A=1\r\nB=2", `OPEN="no close`,
		"BARE={{ K }}", "MID=a{{ dotty://ns/K }}b", "X={{ A }}{{ B }}", "{{",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		lines := Lines(src) // must never panic
		prev := 0
		for _, l := range lines {
			// Line numbers are 1-based and strictly increasing.
			if l.N <= prev {
				t.Errorf("Lines(%q) line numbers not increasing: %d after %d", src, l.N, prev)
			}
			prev = l.N
			// Every key is a valid environment variable name.
			if err := ValidateKey(l.Key); err != nil {
				t.Errorf("Lines(%q) returned invalid key %q: %v", src, l.Key, err)
			}
			// A line is a literal, a single reference, or an error — never
			// a reference-looking literal.
			if l.Err == nil && l.Ref == nil && strings.Contains(l.Value, "{{") {
				t.Errorf("Lines(%q) literal %q still holds a reference opener", src, l.Value)
			}
			if l.Err == nil && l.Ref != nil && ValidateKey(l.Ref.Key) != nil {
				t.Errorf("Lines(%q) reference with invalid key %q", src, l.Ref.Key)
			}
		}
	})
}
