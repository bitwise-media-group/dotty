#! /usr/bin/env zsh

# profile zsh if enabled
if [ -n "${ZSHPROFILE:-}" ]; then
	zmodload zsh/zprof
fi

# ensure that brew is configured
if command -v brew &> /dev/null; then
	eval "$(brew shellenv)"
elif [ -x '/opt/homebrew/bin/brew' ]; then
	eval "$(/opt/homebrew/bin/brew shellenv)"
fi

# set colors for the terminal
export LS_COLORS="$(vivid generate ${VIVID_THEME:-cyberdream})"

if [ -n "${HOMEBREW_PREFIX:-}" ]; then
	google_completion="${HOMEBREW_PREFIX:-}/share/zsh/site-functions/_google_cloud_sdk"

	if [ -f "${google_completion:-}" ]; then
		source "${google_completion:-}"
	fi
fi


if ! (( $+functions[compdef] )); then
	autoload -U +X compinit && compinit
fi

# set the zinit home
if [ -n "${HOMEBREW_PREFIX:-}" ]; then
	ZINIT_HOME="${HOMEBREW_PREFIX}/opt/zinit"
else
	ZINIT_HOME="${XDG_DATA_HOME}/zinit"
fi

# load zinit
source "${ZINIT_HOME}/zinit.zsh"

# add some zinit plugins
zinit light zsh-users/zsh-syntax-highlighting
zinit light zsh-users/zsh-autosuggestions
zinit light Aloxaf/fzf-tab

# add in some snippets
zinit snippet OMZP::sudo
zinit snippet OMZP::command-not-found

zinit cdreplay -q

# setup some key bindings
bindkey -e
bindkey '^p' history-search-backward
bindkey '^n' history-search-forward
bindkey '^[w' kill-region

# setup history
HISTSIZE=5000
HISTFILE=~/.zsh_history
SAVEHIST=$HISTSIZE
HISTDUP=erase
setopt appendhistory
setopt sharehistory
setopt hist_ignore_space
setopt hist_ignore_all_dups
setopt hist_save_no_dups
setopt hist_ignore_dups
setopt hist_find_no_dups

# completion styling
zstyle ':completion:*' matcher-list 'm:{a-z}={A-Za-z}'
zstyle ':completion:*' list-colors "${(s.:.)LS_COLORS}"
zstyle ':completion:*' menu no
zstyle ':fzf-tab:complete:cd:*' fzf-preview 'ls $realpath'
zstyle ':fzf-tab:complete:__zoxide_z:*' fzf-preview 'ls $realpath'

# add aliases; guarded at runtime so this shared file works on machines whose
# profile does not install these tools
if command -v 'lsd' 1>/dev/null 2>&1; then
	alias ls='lsd'
	alias la='ls -a'
	alias lla='ls -la'
	alias ll='ls -l'
	alias lt='ls --tree'
fi

alias sts='dotty tmux new'
alias lts='tmux list-session'
alias kts='tmux kill-server'

if command -v 'nvim' 1>/dev/null 2>&1; then
	alias vim='nvim'
	alias vi='nvim'
fi

# setup the prompt
eval "$(oh-my-posh init zsh --config "${POSH_THEME:-${XDG_CONFIG_HOME}/oh-my-posh/prompt.yaml}")"

# enable shell integrations
eval "$(fzf --zsh)"
eval "$(zoxide init --cmd cd zsh)"
if command -v 'direnv' 1>/dev/null 2>&1; then
	eval "$(direnv hook zsh)"
fi
if command -v 'mise' 1>/dev/null 2>&1; then
	eval "$(mise activate zsh)"
fi

# detect gcloud path
if command -v 'gcloud' 1>/dev/null 2>&1; then
	GOOGLE_BIN_PATH=$(dirname $(readlink -f $(command -v 'gcloud')))
	export PATH="${GOOGLE_BIN_PATH}:${PATH}"
fi

# yazi cd
if command -v 'yazi' 1>/dev/null 2>&1; then
	function y() {
		local tmp="$(mktemp -t "yazi-cwd.XXXXXX")" cwd
		command yazi "$@" --cwd-file="$tmp"
		IFS= read -r -d '' cwd < "$tmp"
		[ "$cwd" != "$PWD" ] && [ -d "$cwd" ] && builtin cd -- "$cwd"
		rm -f -- "$tmp"
	}
fi

# end zsh profiling if enabled
if [ -n "${ZSHPROFILE:-}" ]; then
	zprof
	printf '\n\nTIMINGS:\n\n'
fi
