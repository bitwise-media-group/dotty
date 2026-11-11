// Copyright 2026 Bitwise Media Group Ltd.
// SPDX-License-Identifier: MIT

package worktree

import (
	"path/filepath"
	"strings"
)

// Names are the identifiers derived for a worktree from a repo path and
// suffix. Name doubles as the tmux session name, so it carries the repo
// basename even when the worktree lives inside the repo.
type Names struct {
	Name   string // sanitized "<repo-basename>-<suffix>"
	Branch string // "agent/<name>"
	Path   string // "<root>/<name>"
}

// DefaultRoot is the shared worktree root an empty setting resolves to, and
// the default init stores.
const DefaultRoot = "~/.local/share/dotty/worktrees"

// Root resolves the configured worktree location against a repository: an
// absolute or ~-prefixed setting is a shared root, a relative one is a
// directory inside the repo, and an empty one means DefaultRoot.
func Root(repo, setting, home string) string {
	if setting == "" {
		setting = DefaultRoot
	}
	switch {
	case strings.HasPrefix(setting, "~/"):
		return filepath.Join(home, setting[2:])
	case filepath.IsAbs(setting):
		return setting
	default:
		return filepath.Join(repo, setting)
	}
}

// Derive computes the worktree name, branch, and path under root.
func Derive(repoPath, suffix, root string) Names {
	name := Sanitize(filepath.Base(repoPath) + "-" + suffix)
	return Names{
		Name:   name,
		Branch: "agent/" + name,
		Path:   filepath.Join(root, name),
	}
}

// IsAgentBranch reports whether branch is an agent/* branch — the guard that
// stops `git worktree remove` from deleting a non-agent branch.
func IsAgentBranch(branch string) bool {
	return strings.HasPrefix(branch, "agent/")
}
