// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"fmt"
	"slices"
	"strings"
)

// Platforms are the platforms mise can lock tools for, in its
// `<os>-<arch>[-musl]` naming — the values the lockfile_platforms setting
// and `mise lock --platform` take.
var Platforms = []string{
	"macos-arm64",
	"macos-x64",
	"linux-x64",
	"linux-arm64",
	"linux-x64-musl",
	"linux-arm64-musl",
	"windows-x64",
	"windows-arm64",
}

// IsPlatform reports whether p is one of the Platforms.
func IsPlatform(p string) bool { return slices.Contains(Platforms, p) }

// Platform returns mise's name for goos/goarch (`darwin`/`arm64` is
// `macos-arm64`), assuming a glibc Linux. A combination mise cannot lock for
// returns "".
func Platform(goos, goarch string) string {
	if goos == "darwin" {
		goos = "macos"
	}
	if goarch == "amd64" {
		goarch = "x64"
	}
	if p := goos + "-" + goarch; IsPlatform(p) {
		return p
	}
	return ""
}

// PlatformsTOML formats platforms as the TOML array lockfile_platforms
// takes.
func PlatformsTOML(platforms []string) string {
	quoted := make([]string, len(platforms))
	for i, p := range platforms {
		quoted[i] = fmt.Sprintf("%q", p)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
