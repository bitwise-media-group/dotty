// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package mise

import (
	"fmt"
	"slices"
	"strings"
)

// Converted is one Brewfile entry translated to a mise declaration: the id
// it becomes and the `"<id>" = <value>` line that declares it.
type Converted struct {
	ID   string
	Line string
}

// Conversion is what ConvertBrewfile makes of a Brewfile: `[tools]` and
// `[bootstrap.packages]` declarations, the `[bootstrap.brew.taps]` the
// tapped packages need (keyed by tap, valued by clone URL), and warnings for
// entries mise has no equivalent for.
type Conversion struct {
	Tools    []Converted
	Packages []Converted
	Taps     map[string]string
	Warnings []string
}

// toolForFormula maps Homebrew formulae the template's component fragments
// replaced with locked tools onto their full backend ids, so a Brewfile
// carrying the old template entries converts to what the fragments already
// declare instead of duplicating them as `brew:` packages. Tap-qualified
// formulae are keyed by their full name. The core language runtimes (go,
// dotnet, node) map to mise's own backends.
var toolForFormula = map[string]string{
	// core
	"bat":                                  "aqua:sharkdp/bat",
	"fd":                                   "aqua:sharkdp/fd",
	"fzf":                                  "aqua:junegunn/fzf",
	"gh":                                   "aqua:cli/cli",
	"git-delta":                            "aqua:dandavison/delta",
	"git-lfs":                              "aqua:git-lfs/git-lfs",
	"jq":                                   "aqua:jqlang/jq",
	"ripgrep":                              "aqua:BurntSushi/ripgrep",
	"vivid":                                "aqua:sharkdp/vivid",
	"yq":                                   "aqua:mikefarah/yq",
	"zoxide":                               "aqua:ajeetdsouza/zoxide",
	"jandedobbeleer/oh-my-posh/oh-my-posh": "aqua:JanDeDobbeleer/oh-my-posh",
	"oh-my-posh":                           "aqua:JanDeDobbeleer/oh-my-posh",
	"uv":                                   "aqua:astral-sh/uv",
	// add-ons
	"btop":             "aqua:aristocratos/btop",
	"derailed/k9s/k9s": "aqua:derailed/k9s",
	"k9s":              "aqua:derailed/k9s",
	"lazygit":          "aqua:jesseduffield/lazygit",
	"lsd":              "aqua:lsd-rs/lsd",
	"neovim":           "aqua:neovim/neovim",
	"tmux":             "aqua:tmux/tmux-builds",
	"yazi":             "aqua:sxyazi/yazi",
	"resvg":            "aqua:linebender/resvg",
	// security keys
	"age":                "aqua:FiloSottile/age",
	"age-plugin-yubikey": "github:str4d/age-plugin-yubikey",
	"ykman":              "pipx:yubikey-manager",
	// agents
	"anomalyco/tap/opencode": "aqua:anomalyco/opencode",
	"opencode":               "aqua:anomalyco/opencode",
	// runtimes mise implements itself
	"go":     "go",
	"dotnet": "dotnet",
	"node":   "node",
}

// coreRuntimeTools are the toolForFormula targets that are mise core
// backends rather than fragment entries.
var coreRuntimeTools = []string{"go", "dotnet", "node"}

// toolForUserFormula maps formulae no fragment covers but that mise cannot
// pour as `brew:` packages — Homebrew aliases (`kubectl` is the
// `kubernetes-cli` formula; formulae.brew.sh has no `kubectl.json`) and
// tapped formulae (mise pours a tap's formula only when the tap publishes
// `api/formula/<name>.json`, which hashicorp/tap and fluxcd/tap do not) —
// onto the locked tools that replace them. The plain names are included so
// a Brewfile that names them unqualified converts the same way.
var toolForUserFormula = map[string]string{
	"kubectl":                 "aqua:kubernetes/kubernetes/kubectl",
	"kubernetes-cli":          "aqua:kubernetes/kubernetes/kubectl",
	"hashicorp/tap/terraform": "aqua:hashicorp/terraform",
	"terraform":               "aqua:hashicorp/terraform",
	"fluxcd/tap/flux":         "aqua:fluxcd/flux2",
	"flux":                    "aqua:fluxcd/flux2",
	"awscli":                  "aqua:aws/aws-cli",
	"yt-dlp":                  "github:yt-dlp/yt-dlp",
	"actions-up":              "npm:actions-up",
}

// FragmentToolIDs returns every tool id the converter maps a template-era
// formula or agent cask onto, for the scaffold test that checks each one is
// declared by some fragment.
func FragmentToolIDs() []string {
	ids := slices.Clone(fragmentCaskTools)
	for _, id := range toolForFormula {
		if !slices.Contains(ids, id) && !slices.Contains(coreRuntimeTools, id) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// toolForCask maps casks onto the tools that replace them: the bitwise tap's
// casks become GitHub-release tools (the tap publishes no
// `api/cask/<token>.json`, which mise's brew-cask manager needs, and the
// releases already ship per-platform archives — patchy's archive needs an
// asset match, hence the whole declaration line), and the coding-agent
// CLIs the template's agent fragments now install through the aqua and http
// backends, so a Brewfile from the cask era converts onto the fragments
// instead of duplicating them.
var toolForCask = map[string]Converted{
	"dotty":  {ID: "github:bitwise-media-group/dotty", Line: `"github:bitwise-media-group/dotty" = "latest"`},
	"evolve": {ID: "github:bitwise-media-group/evolve", Line: `"github:bitwise-media-group/evolve" = "latest"`},
	"patchy": {ID: "github:bitwise-media-group/patchy",
		Line: `"github:bitwise-media-group/patchy" = { version = "latest", matching = "patchy-cli" }`},
	"claude-code":        {ID: "aqua:anthropics/claude-code", Line: toolLine("aqua:anthropics/claude-code")},
	"claude-code@latest": {ID: "aqua:anthropics/claude-code", Line: toolLine("aqua:anthropics/claude-code")},
	"codex":              {ID: "aqua:openai/codex", Line: toolLine("aqua:openai/codex")},
	"antigravity": {ID: "aqua:google-antigravity/antigravity-cli",
		Line: toolLine("aqua:google-antigravity/antigravity-cli")},
	"grok-build": {ID: "http:grok", Line: grokToolLine},
}

// grokToolLine is the http-backend declaration for the grok CLI, matching
// the agent-grok fragment: a bare `http:grok` carries no URL, so the line
// spells out the download template and version list the registry's `grok`
// short name expands to.
const grokToolLine = `"http:grok" = { version = "latest", bin = "grok", ` +
	`url = 'https://storage.googleapis.com/grok-build-public-artifacts/cli/` +
	`grok-{{ version }}-{{ os() }}-{{ arch(x64="x86_64", arm64="aarch64") }}', ` +
	`version_list_url = "https://x.ai/cli/stable" }`

// fragmentCaskTools are the toolForCask targets an agent fragment declares;
// the bitwise casks are user-owned and not checked against the fragments.
var fragmentCaskTools = []string{"aqua:anthropics/claude-code", "aqua:openai/codex",
	"aqua:google-antigravity/antigravity-cli", "http:grok"}

// dropFormulae are formulae with no place in the profile: mise itself is
// installed by dotty from the official installer and updates itself.
var dropFormulae = map[string]string{
	"mise": "mise is installed by dotty into ~/.local/bin, not declared",
}

// brewfileEntry is one parsed Brewfile line: the DSL word, the quoted name,
// and the `id:` option App Store entries carry.
type brewfileEntry struct {
	word string
	name string
	id   string
}

// parseBrewfileLine reads `word "name"[, key: value…]` from one line.
// Comments, blanks, and lines that are not entries return ok=false.
func parseBrewfileLine(line string) (brewfileEntry, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return brewfileEntry{}, false
	}
	word, rest, cut := strings.Cut(trimmed, " ")
	if !cut {
		return brewfileEntry{}, false
	}
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, `"`) {
		return brewfileEntry{}, false
	}
	name, opts, closed := strings.Cut(rest[1:], `"`)
	if !closed || name == "" {
		return brewfileEntry{}, false
	}
	e := brewfileEntry{word: word, name: name}
	for opt := range strings.SplitSeq(opts, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(opt), ":")
		if ok && key == "id" {
			e.id = strings.TrimSpace(value)
		}
	}
	return e, true
}

// ConvertBrewfile translates a Brewfile into mise declarations. Formulae the
// template replaced with tools become those tools, other formulae become
// `brew:` packages (taps they reference are recorded), casks become
// `brew-cask:` packages restricted to macOS — except the bitwise tap's,
// which become GitHub-release tools — and the extension entry types map to
// mise's managers and backends (mas, flatpak, go, cargo, npm, uv). Entries
// with no mise equivalent (vscode, krew, unknown words) are reported as
// warnings and dropped.
func ConvertBrewfile(data []byte) Conversion {
	var conv Conversion
	seen := make(map[string]bool)
	add := func(list *[]Converted, c Converted) {
		if seen[c.ID] {
			return
		}
		seen[c.ID] = true
		*list = append(*list, c)
	}
	explicitTaps := make(map[string]bool)
	for line := range strings.Lines(string(data)) {
		e, ok := parseBrewfileLine(line)
		if !ok {
			continue
		}
		switch e.word {
		case "brew":
			if reason, drop := dropFormulae[e.name]; drop {
				conv.Warnings = append(conv.Warnings, fmt.Sprintf("brew %q: %s", e.name, reason))
				continue
			}
			name := strings.ToLower(e.name)
			if id, ok := toolForFormula[name]; ok {
				add(&conv.Tools, Converted{ID: id, Line: toolLine(id)})
				continue
			}
			if id, ok := toolForUserFormula[name]; ok {
				add(&conv.Tools, Converted{ID: id, Line: toolLine(id)})
				continue
			}
			id := "brew:" + e.name
			add(&conv.Packages, Converted{ID: id, Line: fmt.Sprintf("%q = \"latest\"", id)})
			if tap, ok := tapOf(e.name); ok {
				conv.tap(tap)
				conv.Warnings = append(conv.Warnings, fmt.Sprintf(
					"brew %q: mise pours a tapped formula only when %s publishes api/formula/%s.json; "+
						"replace it with a locked tool (`mise registry`) if apply fails",
					e.name, tap, name[strings.LastIndex(name, "/")+1:]))
			}
		case "cask":
			token := e.name
			if parts := strings.SplitN(e.name, "/", 3); len(parts) == 3 {
				token = parts[2]
			}
			if c, ok := toolForCask[strings.ToLower(token)]; ok {
				add(&conv.Tools, c)
				continue
			}
			id := "brew-cask:" + token
			add(&conv.Packages, Converted{ID: id, Line: fmt.Sprintf("%q = { version = \"latest\", os = \"macos\" }", id)})
		case "tap":
			explicitTaps[strings.ToLower(e.name)] = true
		case "mas":
			if e.id == "" {
				conv.Warnings = append(conv.Warnings, fmt.Sprintf("mas %q: no id: option; skipped", e.name))
				continue
			}
			id := "mas:" + e.id
			add(&conv.Packages, Converted{ID: id, Line: fmt.Sprintf("%q = \"latest\" # %s", id, e.name)})
		case "flatpak":
			id := "flatpak:" + e.name
			add(&conv.Packages, Converted{ID: id, Line: fmt.Sprintf("%q = \"latest\"", id)})
		case "go", "cargo", "npm":
			add(&conv.Tools, Converted{ID: e.word + ":" + e.name, Line: toolLine(e.word + ":" + e.name)})
		case "uv":
			add(&conv.Tools, Converted{ID: "pipx:" + e.name, Line: toolLine("pipx:" + e.name)})
		default:
			conv.Warnings = append(conv.Warnings,
				fmt.Sprintf("%s %q: no mise equivalent; add it by hand", e.word, e.name))
		}
	}
	// An explicit tap only matters while a brew: package still references
	// it; taps whose every formula became a tool disappear with them.
	for tap := range explicitTaps {
		if _, referenced := conv.Taps[tap]; !referenced {
			conv.Warnings = append(conv.Warnings, fmt.Sprintf("tap %q: no remaining brew: package uses it; dropped", tap))
		}
	}
	return conv
}

// tap records the tap a brew: package references, with its clone URL.
func (c *Conversion) tap(tap string) {
	if c.Taps == nil {
		c.Taps = make(map[string]string)
	}
	owner, repo, _ := strings.Cut(tap, "/")
	c.Taps[tap] = fmt.Sprintf("https://github.com/%s/homebrew-%s.git", owner, repo)
}

// tapOf returns the owner/repo tap a tap-qualified formula name belongs to;
// unqualified and homebrew/* names belong to none.
func tapOf(name string) (string, bool) {
	parts := strings.Split(strings.ToLower(name), "/")
	if len(parts) != 3 || parts[0] == "homebrew" {
		return "", false
	}
	return parts[0] + "/" + strings.TrimPrefix(parts[1], "homebrew-"), true
}

// toolLine renders one `[tools]` declaration at "latest": backend ids are
// quoted keys, mise core tools (go, node) bare ones, the way mise writes them.
func toolLine(id string) string {
	if strings.Contains(id, ":") {
		return fmt.Sprintf("%q = \"latest\"", id)
	}
	return id + ` = "latest"`
}

// TapLine renders one `[bootstrap.brew.taps]` declaration.
func TapLine(tap, url string) string { return fmt.Sprintf("%q = %q", tap, url) }
