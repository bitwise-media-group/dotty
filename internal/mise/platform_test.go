// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import "testing"

func TestPlatform(t *testing.T) {
	for _, tc := range []struct{ goos, goarch, want string }{
		{"darwin", "arm64", "macos-arm64"},
		{"darwin", "amd64", "macos-x64"},
		{"linux", "amd64", "linux-x64"},
		{"linux", "arm64", "linux-arm64"},
		{"windows", "amd64", "windows-x64"},
		{"linux", "386", ""},
		{"freebsd", "amd64", ""},
	} {
		if got := Platform(tc.goos, tc.goarch); got != tc.want {
			t.Errorf("Platform(%s, %s) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}

func TestPlatformsTOML(t *testing.T) {
	for _, tc := range []struct {
		platforms []string
		want      string
	}{
		{nil, "[]"},
		{[]string{"macos-arm64"}, `["macos-arm64"]`},
		{[]string{"macos-arm64", "linux-x64-musl"}, `["macos-arm64", "linux-x64-musl"]`},
	} {
		if got := PlatformsTOML(tc.platforms); got != tc.want {
			t.Errorf("PlatformsTOML(%v) = %s, want %s", tc.platforms, got, tc.want)
		}
	}
}
