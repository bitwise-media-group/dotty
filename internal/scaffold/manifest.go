// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package scaffold

// sharedDoc is the one source for the per-agent memory documents
// (CLAUDE.md/AGENTS.md); sharedDocOps fans it out to each selected agent.
const sharedDoc = "template/shared/AGENTS.md"

// Component is one selectable unit of the template: the embedded paths it
// contributes, its mise fragment, and the $HOME directories that must
// stay real directories (Unfold) so tools writing runtime state beside their
// config never write through a folded symlink into the repository.
type Component struct {
	ID       string
	Prefixes []string          // embedded dirs/files copied to the same repo-relative path
	Renames  map[string]string // embedded path → repo-relative destination
	Mise     string            // embedded mise conf.d fragment (tools and bootstrap packages)
	Unfold   []string          // $HOME-relative dirs pre-created as real dirs
	Doc      string            // repo-relative home of the shared agent doc
}

// CopyDirs lists the $HOME-relative directories the linker deploys as real
// directories of file copies instead of symlinks. Grok's sandbox
// kernel-denies writes to its hooks directory and refuses to start when that
// directory (or any hook path component) is a symlink — a symlink would let
// sandboxed code retarget the hooks past the deny. The list is static rather
// than per-component: a path whose component is unselected never exists under
// the repository's home/ tree, so the linker simply never matches it.
var CopyDirs = []string{".config/grok/hooks"}

// templated lists the embedded files rendered through text/template; every
// other file is copied byte-for-byte, so configs whose own syntax uses {{ }}
// stay untouched. The hardening phase adds more agent configs here when they
// grow {{if .Harden}} blocks.
var templated = map[string]bool{
	"template/repo/dotty-marker":                    true,
	"template/machine/env.zsh":                      true,
	"template/machine/git.gitconfig":                true,
	"template/machine/worktrees.gitconfig":          true,
	"template/home/.config/git/ignore":              true,
	"template/home/.config/claude/settings.json":    true,
	"template/home/.config/codex/dotty.config.toml": true,
	"template/home/.config/opencode/opencode.jsonc": true,
	"template/home/.config/grok/config.toml":        true,
	"template/home/.config/grok/sandbox.toml":       true,
}

// machine lists the embedded files whose rendered output varies per machine
// (paths, selections, marketplace). They render into the profile's directory
// rather than the shared repository, and reach $HOME (or an
// include) only through the active-profile symlink — so activating another
// profile swaps them all at once, and the shared repository never carries a
// machine-specific byte.
var perProfile = map[string]bool{
	"template/mise/config.toml":                     true,
	"template/machine/env.zsh":                      true,
	"template/machine/git.gitconfig":                true,
	"template/machine/worktrees.gitconfig":          true,
	"template/home/.config/claude/settings.json":    true,
	"template/home/.config/codex/dotty.config.toml": true,
	"template/home/.config/opencode/opencode.jsonc": true,
	"template/home/.config/grok/config.toml":        true,
	"template/home/.config/grok/sandbox.toml":       true,
}

// keepExisting lists the embedded files rendered once and then owned by the
// user: the profile's mise config.toml, which `dotty packages add` and
// `mise use -g` write into, so a re-render must not reset it.
var keepExisting = map[string]bool{
	"template/mise/config.toml": true,
}

// executable lists the embedded files written 0o755 — go:embed does not
// carry mode bits.
var executable = map[string]bool{
	"template/home/.config/git/template/hooks/prepare-commit-msg": true,
	"template/home/.config/codex/hooks/pre-tool-use-policy":       true,
	"template/home/.config/grok/hooks/pre-tool-use-policy":        true,
}

// manifest maps wizard selections onto the template. IDs are what selected()
// derives from Answers: core, addon:<name>, agent:<name>,
// feature:security-keys.
var manifest = []Component{
	{
		ID: "core",
		Prefixes: []string{
			"template/home/.config/ghostty",
			"template/home/.config/oh-my-posh/prompt.yaml",
			"template/home/.config/vivid",
			"template/home/.config/zsh",
			"template/home/.config/git",
			"template/home/.terminfo",
			"template/home/.zshenv",
		},
		Renames: map[string]string{
			"template/repo/CREDITS.md":             "CREDITS.md",
			"template/repo/dotty-marker":           ".dotty-version",
			"template/machine/env.zsh":             "env.zsh",
			"template/machine/git.gitconfig":       "git.gitconfig",
			"template/machine/worktrees.gitconfig": "worktrees.gitconfig",
			"template/mise/config.toml":            "mise/config.toml",
		},
		Mise: "template/mise.d/core.toml",
		// .config/dotty stays a real directory so the machine-local
		// active-profile symlink lives beside the repo-linked profiles.
		Unfold: []string{".config", ".config/dotty", ".config/zsh"},
	},

	{
		ID:       "addon:btop",
		Prefixes: []string{"template/home/.config/btop"},
		Mise:     "template/mise.d/addon-btop.toml",
	},
	{
		ID:       "addon:k9s",
		Prefixes: []string{"template/home/.config/k9s"},
		Mise:     "template/mise.d/addon-k9s.toml",
	},
	{
		ID:       "addon:lazygit",
		Prefixes: []string{"template/home/.config/lazygit"},
		Mise:     "template/mise.d/addon-lazygit.toml",
	},
	{
		ID:       "addon:lsd",
		Prefixes: []string{"template/home/.config/lsd"},
		Mise:     "template/mise.d/addon-lsd.toml",
	},
	{
		ID:       "addon:tmux",
		Prefixes: []string{"template/home/.config/tmux"},
		Mise:     "template/mise.d/addon-tmux.toml",
	},
	{
		ID:       "addon:yazi",
		Prefixes: []string{"template/home/.config/yazi"},
		Mise:     "template/mise.d/addon-yazi.toml",
	},
	{
		// The issue couples nvim with lazygit; the fragment carries both.
		ID:       "addon:nvim",
		Prefixes: []string{"template/home/.config/nvim", "template/home/.config/lazygit"},
		Mise:     "template/mise.d/addon-nvim.toml",
	},

	{
		ID:       "agent:claude-code",
		Prefixes: []string{"template/home/.config/claude"},
		Renames:  map[string]string{"template/home/.config/oh-my-posh/claude.yaml": "home/.config/oh-my-posh/claude.yaml"},
		Mise:     "template/mise.d/agent-claude-code.toml",
		Unfold:   []string{".config/claude"},
		Doc:      "home/.config/claude/CLAUDE.md",
	},
	{
		ID:       "agent:codex",
		Prefixes: []string{"template/home/.config/codex"},
		Mise:     "template/mise.d/agent-codex.toml",
		Unfold:   []string{".config/codex"},
		Doc:      "home/.config/codex/AGENTS.md",
	},
	{
		ID:       "agent:opencode",
		Prefixes: []string{"template/home/.config/opencode"},
		Mise:     "template/mise.d/agent-opencode.toml",
		Unfold:   []string{".config/opencode"},
		Doc:      "home/.config/opencode/AGENTS.md",
	},
	{
		// No dotfiles exist for antigravity upstream; only its CLI is installed.
		ID:   "agent:antigravity",
		Mise: "template/mise.d/agent-antigravity.toml",
	},
	{
		ID:       "agent:grok",
		Prefixes: []string{"template/home/.config/grok"},
		Mise:     "template/mise.d/agent-grok.toml",
		Unfold:   []string{".config/grok"},
		Doc:      "home/.config/grok/AGENTS.md",
	},

	{
		ID:       "feature:security-keys",
		Prefixes: []string{"template/home/.ssh"},
		Mise:     "template/mise.d/feature-security-keys.toml",
		Unfold:   []string{".ssh"},
	},
}
