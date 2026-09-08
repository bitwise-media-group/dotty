// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// TestConvertBrewfile pins the per-entry mapping.
func TestConvertBrewfile(t *testing.T) {
	cases := []struct {
		name         string
		line         string
		wantTools    []string // ids
		wantPackages []string // lines
		wantTaps     []string
		wantWarnings int
	}{
		{name: "template formula becomes its tool", line: `brew "ripgrep"`, wantTools: []string{"aqua:BurntSushi/ripgrep"}},
		{name: "tap-qualified template formula", line: `brew "derailed/k9s/k9s", trusted: true`,
			wantTools: []string{"aqua:derailed/k9s"}},
		{name: "options do not matter", line: `brew "jandedobbeleer/oh-my-posh/oh-my-posh", args: ["formula"], trusted: true`,
			wantTools: []string{"aqua:JanDeDobbeleer/oh-my-posh"}},
		{name: "runtime formula", line: `brew "go"`, wantTools: []string{"go"}},
		{name: "other formula becomes brew package", line: `brew "colima"`,
			wantPackages: []string{`"brew:colima" = "latest"`}},
		{name: "tapped formula records its tap and warns", line: `brew "acme/tap/widget", trusted: true`,
			wantPackages: []string{`"brew:acme/tap/widget" = "latest"`},
			wantTaps:     []string{`"acme/tap" = "https://github.com/acme/homebrew-tap.git"`}, wantWarnings: 1},
		{name: "tapped formula with a locked equivalent", line: `brew "hashicorp/tap/terraform", trusted: true`,
			wantTools: []string{"aqua:hashicorp/terraform"}},
		{name: "brew alias with a locked equivalent", line: `brew "kubectl"`,
			wantTools: []string{"aqua:kubernetes/kubernetes/kubectl"}},
		{name: "mise is dropped", line: `brew "mise"`, wantWarnings: 1},
		{name: "cask becomes macos package", line: `cask "ghostty"`,
			wantPackages: []string{`"brew-cask:ghostty" = { version = "latest", os = "macos" }`}},
		{name: "versioned cask keeps its brew name", line: `cask "temurin@17"`,
			wantPackages: []string{`"brew-cask:temurin@17" = { version = "latest", os = "macos" }`}},
		{name: "agent cask becomes its fragment tool", line: `cask "claude-code@latest"`,
			wantTools: []string{"aqua:anthropics/claude-code"}},
		{name: "grok cask becomes the http tool", line: `cask "grok-build"`, wantTools: []string{"http:grok"}},
		{name: "bitwise cask becomes github tool", line: `cask "bitwise-media-group/tap/patchy", trusted: true`,
			wantTools: []string{"github:bitwise-media-group/patchy"}},
		{name: "unreferenced tap warns", line: `tap "bitwise-media-group/tap", trusted: true`, wantWarnings: 1},
		{name: "mas", line: `mas "Xcode", id: 497799835`, wantPackages: []string{`"mas:497799835" = "latest" # Xcode`}},
		{name: "mas without id warns", line: `mas "Xcode"`, wantWarnings: 1},
		{name: "flatpak", line: `flatpak "org.mozilla.firefox"`, wantPackages: []string{`"flatpak:org.mozilla.firefox" = "latest"`}},
		{name: "go", line: `go "golang.org/x/tools/gopls"`, wantTools: []string{"go:golang.org/x/tools/gopls"}},
		{name: "cargo", line: `cargo "ripgrep"`, wantTools: []string{"cargo:ripgrep"}},
		{name: "npm", line: `npm "prettier"`, wantTools: []string{"npm:prettier"}},
		{name: "uv becomes pipx", line: `uv "ruff"`, wantTools: []string{"pipx:ruff"}},
		{name: "vscode warns", line: `vscode "golang.go"`, wantWarnings: 1},
		{name: "krew warns", line: `krew "ctx"`, wantWarnings: 1},
		{name: "comment ignored", line: `# brew "ripgrep"`},
		{name: "duplicates collapse", line: "brew \"bat\"\nbrew \"bat\"", wantTools: []string{"aqua:sharkdp/bat"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conv := ConvertBrewfile([]byte(c.line + "\n"))
			var tools, packages, taps []string
			for _, e := range conv.Tools {
				tools = append(tools, e.ID)
			}
			for _, e := range conv.Packages {
				packages = append(packages, e.Line)
			}
			for tap, url := range conv.Taps {
				taps = append(taps, TapLine(tap, url))
			}
			if !slices.Equal(tools, c.wantTools) {
				t.Errorf("tools = %v, want %v", tools, c.wantTools)
			}
			if !slices.Equal(packages, c.wantPackages) {
				t.Errorf("packages = %v, want %v", packages, c.wantPackages)
			}
			if !slices.Equal(taps, c.wantTaps) {
				t.Errorf("taps = %v, want %v", taps, c.wantTaps)
			}
			if len(conv.Warnings) != c.wantWarnings {
				t.Errorf("warnings = %v, want %d", conv.Warnings, c.wantWarnings)
			}
		})
	}
}

// TestConvertRealBrewfile runs the converter over a real profile Brewfile
// and pins the migration outcome: the template formulae and casks vanish
// into tools, the machine's own additions become packages, and the bitwise
// tap disappears with its casks.
func TestConvertRealBrewfile(t *testing.T) {
	data, err := os.ReadFile("testdata/Brewfile")
	if err != nil {
		t.Fatal(err)
	}
	conv := ConvertBrewfile(data)
	ids := make(map[string]bool)
	for _, e := range conv.Tools {
		ids[e.ID] = true
	}
	for _, e := range conv.Packages {
		ids[e.ID] = true
	}
	for _, want := range []string{
		"aqua:sharkdp/bat", "aqua:JanDeDobbeleer/oh-my-posh", "aqua:derailed/k9s", "aqua:neovim/neovim",
		"github:bitwise-media-group/dotty", "github:bitwise-media-group/evolve", "github:bitwise-media-group/patchy",
		"go", "dotnet", "aqua:astral-sh/uv", "pipx:yubikey-manager",
		"aqua:hashicorp/terraform", "aqua:fluxcd/flux2", "aqua:kubernetes/kubernetes/kubectl", "aqua:aws/aws-cli",
		"brew:git", "brew:colima", "brew:docker", "brew:docker-compose",
		"brew-cask:ghostty", "brew-cask:raycast", "brew-cask:font-iosevka-nerd-font",
		"aqua:anthropics/claude-code", "aqua:openai/codex", "http:grok",
	} {
		if !ids[want] {
			t.Errorf("%s missing from the conversion", want)
		}
	}
	for _, e := range conv.Tools {
		if e.ID == "go" && e.Line != `go = "latest"` {
			t.Errorf("core tool line = %q, want a bare key", e.Line)
		}
	}
	for _, gone := range []string{"brew:mise", "brew:bat", "brew-cask:dotty", "brew-cask:patchy", "brew:go",
		"brew:kubectl", "brew:hashicorp/tap/terraform", "brew:fluxcd/tap/flux",
		"brew-cask:claude-code@latest", "brew-cask:codex", "brew-cask:grok-build"} {
		if ids[gone] {
			t.Errorf("%s should not survive the conversion", gone)
		}
	}
	if len(conv.Taps) != 0 {
		t.Errorf("taps = %v, want none once the tapped formulae became tools", conv.Taps)
	}
	joined := strings.Join(conv.Warnings, "\n")
	for _, want := range []string{`tap "bitwise-media-group/tap"`, `brew "mise"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("warnings missing %s:\n%s", want, joined)
		}
	}
	if len(conv.Warnings) != 2 {
		t.Errorf("warnings = %v, want exactly the tap and mise", conv.Warnings)
	}
}
