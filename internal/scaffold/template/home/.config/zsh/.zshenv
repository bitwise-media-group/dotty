export XDG_CONFIG_HOME="${HOME}/.config"
export XDG_CACHE_HOME="${HOME}/.cache"
export XDG_DATA_HOME="${HOME}/.local/share"
export XDG_STATE_HOME="${HOME}/.local/state"
export XDG_RUNTIME_DIR="${HOME}/.local/runtime"

export REPOS_DIR="${HOME}/Repos"

export TMUX_SOCK="${TMUX%%,*}"

export HOMEBREW_BUNDLE_FILE_GLOBAL="${XDG_DATA_HOME}/homebrew/Brewfile"
export HOMEBREW_BUNDLE_FORCE_INSTALL_CLEANUP=1
export HOMEBREW_REQUIRE_TAP_TRUST=1

export GOPATH="${XDG_DATA_HOME}/go"
export GOCACHE="${XDG_CACHE_HOME}/go/build"
export GOMODCACHE="${XDG_CACHE_HOME}/go/mod"
export GOENV="${XDG_CACHE_HOME}/go/env"
export GOLANGCI_LINT_CACHE="${XDG_CACHE_HOME}/golangci-lint"

export POSH_THEME="${XDG_CONFIG_HOME}/oh-my-posh/prompt.yaml"
export VIVID_THEME="${XDG_CONFIG_HOME}/vivid/themes/cyberdream.yaml"

export EDITOR=vim

# Machine-specific overrides (REPOS_DIR, EDITOR, CODEX_HOME, …) come from the
# active dotty profile, so switching profiles retargets them without touching
# this shared file. `dotty init` renders it; missing is fine.
if [ -f "${XDG_CONFIG_HOME}/dotty/active-profile/env.zsh" ]; then
	. "${XDG_CONFIG_HOME}/dotty/active-profile/env.zsh"
fi

if [ "${TERM_PROGRAM}" = "vscode" ]; then
	export EDITOR='code --wait'
fi

# Route OpenSSH PIN prompts through dotty's ask-pass bridge → pinentry-mac,
# which caches the security-key PIN in the macOS keychain (shared by git
# signing and ssh auth with an sk identity file). Guarded on the symlink so
# a shell never breaks before `dotty init` has created it.
if [ -e "${XDG_DATA_HOME}/dotty/dotty-ssh-askpass" ]; then
	export SSH_ASKPASS="${XDG_DATA_HOME}/dotty/dotty-ssh-askpass"
	export SSH_ASKPASS_REQUIRE=force
fi
