export XDG_CONFIG_HOME="${HOME}/.config"
export XDG_CACHE_HOME="${HOME}/.cache"
export XDG_DATA_HOME="${HOME}/.local/share"
export XDG_STATE_HOME="${HOME}/.local/state"
export XDG_RUNTIME_DIR="${HOME}/.local/runtime"

export REPOS_DIR="${HOME}/Repos"

export TMUX_SOCK="${TMUX%%,*}"

# PATH for every shell, interactive or not: the Homebrew prefix mise pours
# bootstrap packages into (HOMEBREW_PREFIX also steers .zshrc's gcloud
# completion and ZINIT_HOME), then mise's shims so scripts see the locked
# tools, then ~/.local/bin where dotty installs mise itself. Interactive
# shells replace the shims with `mise activate` in .zshrc.
for __prefix in /opt/homebrew /home/linuxbrew/.linuxbrew; do
	if [ -d "${__prefix}/bin" ]; then
		export HOMEBREW_PREFIX="${__prefix}"
		export PATH="${__prefix}/bin:${__prefix}/sbin:${PATH}"
		break
	fi
done
unset __prefix
export PATH="${XDG_DATA_HOME}/mise/shims:${HOME}/.local/bin:${PATH}"

# ~/.config/mise is a symlink into the active dotty profile; trusting the
# profile tree keeps mise from asking about the global config and its
# conf.d fragments (mise canonicalises paths before the check).
export MISE_TRUSTED_CONFIG_PATHS="${XDG_CONFIG_HOME}/dotty"

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

# Route OpenSSH prompts through dotty's ask-pass bridge → pinentry-mac. The
# force setting sends every prompt there, so the bridge dispatches by kind:
# PIN entries get a cached-in-keychain GETPIN (shared by git signing and ssh
# auth with an sk identity file), yes/no questions like the host-authenticity
# check get a CONFIRM dialog. Guarded on the symlink so a shell never breaks
# before `dotty init` has created it.
if [ -e "${XDG_DATA_HOME}/dotty/dotty-ssh-askpass" ]; then
	export SSH_ASKPASS="${XDG_DATA_HOME}/dotty/dotty-ssh-askpass"
	export SSH_ASKPASS_REQUIRE=force
fi
