// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"path/filepath"
	"testing"
)

func TestHomeRel(t *testing.T) {
	home := "/Users/u"
	cases := []struct {
		name, in, want string
		wantErr        bool
	}{
		{"absolute inside home", "/Users/u/.ssh/known_hosts", ".ssh/known_hosts", false},
		{"already relative", ".config/private/git/config", ".config/private/git/config", false},
		{"relative cleaned", "./.ssh//config", ".ssh/config", false},
		{"outside home", "/etc/hosts", "", true},
		{"escapes home", "/Users/u/../other/file", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := homeRel(tc.in, home)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("homeRel(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("homeRel(%q): %v", tc.in, err)
			}
			if got != filepath.FromSlash(tc.want) {
				t.Errorf("homeRel(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
