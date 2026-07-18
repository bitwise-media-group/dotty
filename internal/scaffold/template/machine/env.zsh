# Copyright 2026 BitWise Media Group Ltd
# SPDX-License-Identifier: MIT

# Machine-specific environment for profile {{.ProfileName}}, rendered by
# `dotty init` and sourced by the shared .zshenv through the active-profile
# symlink — activating a different profile swaps these values.
export REPOS_DIR="{{.ReposDirShell}}"

# Where agent worktrees live: a directory name inside each repository, or an
# absolute path for one shared root. dotty worktree and the nvim session
# picker read this.
export DOTTY_WORKTREES="{{.Worktrees}}"
{{- if .HasNvim }}
export EDITOR=nvim
{{- end }}
{{- if .HasCodex }}

# Relocate Codex's home under XDG config so its stow package lives at
# .config/codex/ like every other tool (Codex defaults to ~/.codex), and its
# sqlite state under XDG data so runtime databases never clobber the
# dotfiles-linked config dir.
export CODEX_HOME="${XDG_CONFIG_HOME}/codex"
export CODEX_SQLITE_HOME="${XDG_DATA_HOME}/codex"
{{- end }}
{{- if .HasClaude }}

# Relocate Claude Code's config under XDG config (it defaults to ~/.claude),
# and its plugin cache under XDG cache so marketplace clones never land in
# the dotfiles-linked config dir.
export CLAUDE_CONFIG_DIR="${XDG_CONFIG_HOME}/claude"
export CLAUDE_CODE_PLUGIN_CACHE_DIR="${XDG_CACHE_HOME}/claude/plugins"
{{- end }}
{{- if .HasGrok }}

# Relocate Grok's home under XDG config (it defaults to ~/.grok).
export GROK_HOME="${XDG_CONFIG_HOME}/grok"
{{- end }}
