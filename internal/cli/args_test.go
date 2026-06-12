// MIT License
//
// Copyright (c) 2026 Bitwise Media Group
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cli

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// signSpec mirrors the owned-flag spec the signing-key sign proxy uses.
var signSpec = map[string]bool{
	"security-key":     true,
	"username":         true,
	"print-git-config": false,
}

func TestExtractFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOwn  map[string]string
		wantRest []string
		wantHelp bool
	}{
		{
			name:     "empty argv",
			args:     nil,
			wantOwn:  map[string]string{},
			wantRest: []string{},
		},
		{
			name:     "git sign argv passes through untouched",
			args:     []string{"-Y", "sign", "-n", "git", "-f", "/tmp/key", "/tmp/buffer"},
			wantOwn:  map[string]string{},
			wantRest: []string{"-Y", "sign", "-n", "git", "-f", "/tmp/key", "/tmp/buffer"},
		},
		{
			name:     "equals form extracted anywhere",
			args:     []string{"-Y", "sign", "--security-key=work", "-n", "git"},
			wantOwn:  map[string]string{"security-key": "work"},
			wantRest: []string{"-Y", "sign", "-n", "git"},
		},
		{
			name:     "space form consumes the next arg",
			args:     []string{"--username", "deavon", "file.txt"},
			wantOwn:  map[string]string{"username": "deavon"},
			wantRest: []string{"file.txt"},
		},
		{
			name:     "boolean owned flag",
			args:     []string{"--print-git-config"},
			wantOwn:  map[string]string{"print-git-config": "true"},
			wantRest: []string{},
		},
		{
			name:     "help short form intercepted",
			args:     []string{"-h"},
			wantOwn:  map[string]string{},
			wantRest: []string{},
			wantHelp: true,
		},
		{
			name:     "help long form intercepted among passthrough",
			args:     []string{"-Y", "sign", "--help"},
			wantOwn:  map[string]string{},
			wantRest: []string{"-Y", "sign"},
			wantHelp: true,
		},
		{
			name:     "double dash stops scanning and forwards everything",
			args:     []string{"--username=u", "--", "--help", "--security-key=x"},
			wantOwn:  map[string]string{"username": "u"},
			wantRest: []string{"--", "--help", "--security-key=x"},
		},
		{
			name:     "unknown long flags forward verbatim",
			args:     []string{"--vvv", "--other=1"},
			wantOwn:  map[string]string{},
			wantRest: []string{"--vvv", "--other=1"},
		},
		{
			name:     "value flag at end of argv records empty value",
			args:     []string{"--username"},
			wantOwn:  map[string]string{"username": ""},
			wantRest: []string{},
		},
		{
			name:     "bare double dash only",
			args:     []string{"--"},
			wantOwn:  map[string]string{},
			wantRest: []string{"--"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			own, rest, help := ExtractFlags(tt.args, signSpec)
			if !maps.Equal(own, tt.wantOwn) {
				t.Errorf("own = %v, want %v", own, tt.wantOwn)
			}
			if !slices.Equal(rest, tt.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tt.wantRest)
			}
			if help != tt.wantHelp {
				t.Errorf("help = %v, want %v", help, tt.wantHelp)
			}
		})
	}
}

func FuzzExtractFlags(f *testing.F) {
	f.Add("-Y sign -n git -f /tmp/key /tmp/buffer")
	f.Add("--security-key=work --username deavon -- --help")
	f.Add("--print-git-config -h --")
	f.Add("--username")
	f.Fuzz(func(t *testing.T, input string) {
		args := strings.Fields(input)
		own, rest, _ := ExtractFlags(args, signSpec)

		if len(rest) > len(args) {
			t.Fatalf("rest grew: %d args in, %d out", len(args), len(rest))
		}
		// Every forwarded arg must appear in the input in order — the proxy
		// must never invent or reorder argv.
		i := 0
		for _, r := range rest {
			found := false
			for ; i < len(args); i++ {
				if args[i] == r {
					found = true
					i++
					break
				}
			}
			if !found {
				t.Fatalf("rest arg %q not found in order in input %v", r, args)
			}
		}
		// Owned flags must be a subset of the spec.
		for name := range own {
			if _, ok := signSpec[name]; !ok {
				t.Fatalf("extracted flag %q not in spec", name)
			}
		}
	})
}
