// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package env

import (
	"reflect"
	"strings"
	"testing"
)

func TestCapture(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		wantOut     string
		wantSecrets map[string]string
	}{
		{
			name:        "simple assignment",
			src:         "TOKEN=abc123",
			wantOut:     "TOKEN={{ dotty://ci/TOKEN }}",
			wantSecrets: map[string]string{"TOKEN": "abc123"},
		},
		{
			name:        "export prefix and spacing preserved",
			src:         "export  AWS_KEY = secret",
			wantOut:     "export  AWS_KEY = {{ dotty://ci/AWS_KEY }}",
			wantSecrets: map[string]string{"AWS_KEY": "secret"},
		},
		{
			name:        "indentation preserved",
			src:         "\tNESTED=value",
			wantOut:     "\tNESTED={{ dotty://ci/NESTED }}",
			wantSecrets: map[string]string{"NESTED": "value"},
		},
		{
			name:        "double quotes are decoded",
			src:         `MSG="line1\nline2"`,
			wantOut:     "MSG={{ dotty://ci/MSG }}",
			wantSecrets: map[string]string{"MSG": "line1\nline2"},
		},
		{
			name:        "single quotes are literal",
			src:         `RAW='a\nb'`,
			wantOut:     "RAW={{ dotty://ci/RAW }}",
			wantSecrets: map[string]string{"RAW": `a\nb`},
		},
		{
			name:        "inline comment preserved",
			src:         "KEY=value # trailing note",
			wantOut:     "KEY={{ dotty://ci/KEY }} # trailing note",
			wantSecrets: map[string]string{"KEY": "value"},
		},
		{
			name:        "hash without leading space stays in value",
			src:         "PASS=p#ss",
			wantOut:     "PASS={{ dotty://ci/PASS }}",
			wantSecrets: map[string]string{"PASS": "p#ss"},
		},
		{
			name:        "comment after quoted value preserved",
			src:         `Q="v" # note`,
			wantOut:     "Q={{ dotty://ci/Q }} # note",
			wantSecrets: map[string]string{"Q": "v"},
		},
		{
			name:        "value with equals sign",
			src:         "PAIR=a=b=c",
			wantOut:     "PAIR={{ dotty://ci/PAIR }}",
			wantSecrets: map[string]string{"PAIR": "a=b=c"},
		},
		{
			name:        "comments and blanks untouched",
			src:         "# a comment\n\nKEY=v\n",
			wantOut:     "# a comment\n\nKEY={{ dotty://ci/KEY }}\n",
			wantSecrets: map[string]string{"KEY": "v"},
		},
		{
			name:        "empty value left as is",
			src:         "EMPTY=\nQUOTED_EMPTY=\"\"",
			wantOut:     "EMPTY=\nQUOTED_EMPTY=\"\"",
			wantSecrets: map[string]string{},
		},
		{
			name:        "existing reference is idempotent",
			src:         "TOKEN={{ dotty://ci/TOKEN }}",
			wantOut:     "TOKEN={{ dotty://ci/TOKEN }}",
			wantSecrets: map[string]string{},
		},
		{
			name:        "non-assignment lines passthrough",
			src:         "not an assignment\n1BAD=x",
			wantOut:     "not an assignment\n1BAD=x",
			wantSecrets: map[string]string{},
		},
		{
			name:        "unterminated quote left untouched",
			src:         `OPEN="no close`,
			wantOut:     `OPEN="no close`,
			wantSecrets: map[string]string{},
		},
		{
			name:        "crlf endings preserved",
			src:         "A=1\r\nB=2\r\n",
			wantOut:     "A={{ dotty://ci/A }}\r\nB={{ dotty://ci/B }}\r\n",
			wantSecrets: map[string]string{"A": "1", "B": "2"},
		},
		{
			name:        "last duplicate wins",
			src:         "DUP=first\nDUP=second",
			wantOut:     "DUP={{ dotty://ci/DUP }}\nDUP={{ dotty://ci/DUP }}",
			wantSecrets: map[string]string{"DUP": "second"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, secrets, err := Capture(tt.src, "ci")
			if err != nil {
				t.Fatalf("Capture(%q): %v", tt.src, err)
			}
			if out != tt.wantOut {
				t.Errorf("Capture(%q) out =\n%q\nwant\n%q", tt.src, out, tt.wantOut)
			}
			if !reflect.DeepEqual(secrets, tt.wantSecrets) {
				t.Errorf("Capture(%q) secrets = %v, want %v", tt.src, secrets, tt.wantSecrets)
			}
		})
	}
}

func TestCaptureInvalidNamespace(t *testing.T) {
	if _, _, err := Capture("KEY=v", "bad:ns"); err == nil {
		t.Fatal("Capture with invalid namespace returned nil error")
	}
}

// TestCaptureInjectRoundTrip checks that Capture and Inject are inverses: the
// references Capture writes resolve, via the secrets it collected, back to the
// source. Values are unquoted here so decoding is the identity and the text
// round-trips exactly; quoted values intentionally decode to their literal
// secret (covered by TestCaptureDecodesQuotedValue).
func TestCaptureInjectRoundTrip(t *testing.T) {
	src := "export A=alpha\nB = two-words # note\nC=literal\n"
	out, secrets, err := Capture(src, "ns")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	resolved, err := Inject(out, func(ns, key string) (string, error) {
		if ns != "ns" {
			t.Errorf("resolver got namespace %q, want ns", ns)
		}
		return secrets[key], nil
	})
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if resolved != src {
		t.Errorf("round trip = %q, want %q", resolved, src)
	}
}

// TestCaptureDecodesQuotedValue documents that the stored secret is the decoded
// value: .env quoting and escapes are stripped so the keychain holds the real
// value that env run / env use will emit.
func TestCaptureDecodesQuotedValue(t *testing.T) {
	_, secrets, err := Capture(`B="two\twords"`, "ns")
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if got := secrets["B"]; got != "two\twords" {
		t.Errorf("stored value = %q, want %q", got, "two\twords")
	}
}

// TestCaptureIdempotent checks that capturing already-captured output is a
// no-op: the text is unchanged and nothing new is collected.
func TestCaptureIdempotent(t *testing.T) {
	src := "A=1\nB=2\n"
	once, _, err := Capture(src, "ns")
	if err != nil {
		t.Fatal(err)
	}
	twice, secrets, err := Capture(once, "ns")
	if err != nil {
		t.Fatal(err)
	}
	if twice != once {
		t.Errorf("second Capture = %q, want unchanged %q", twice, once)
	}
	if len(secrets) != 0 {
		t.Errorf("second Capture collected %v, want none", secrets)
	}
}

func FuzzCapture(f *testing.F) {
	for _, seed := range []string{
		"K=v", "export K = v # c", `Q="a\nb"`, "K={{ dotty://ns/K }}",
		"# comment", "", "K=", "1BAD=x", "A=1\r\nB=2", `OPEN="no close`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		out, secrets, err := Capture(src, "ns") // must never panic
		if err != nil {
			return
		}
		// Every captured key is rewritten to its reference in the output.
		for key := range secrets {
			ref := "{{ dotty://ns/" + key + " }}"
			if !strings.Contains(out, ref) {
				t.Errorf("Capture(%q) captured %q but output %q lacks %q", src, key, out, ref)
			}
		}
		// Capturing the output again is a no-op: references are recognized and
		// left in place, so nothing new is collected.
		again, more, err := Capture(out, "ns")
		if err != nil {
			t.Fatalf("second Capture(%q): %v", out, err)
		}
		if again != out {
			t.Errorf("Capture not idempotent: %q -> %q", out, again)
		}
		if len(more) != 0 {
			t.Errorf("second Capture collected %v, want none", more)
		}
	})
}
