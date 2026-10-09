# macOS's /etc/zprofile runs path_helper in every login shell (what Terminal
# and Ghostty open), after .zshenv has built PATH. path_helper puts /etc/paths
# (/usr/bin, /bin, …) first and demotes everything else, so system binaries
# like /usr/bin/ssh-keygen would shadow the Homebrew prefix's. Move .zshenv's
# entries back to the front; typeset -U drops their demoted duplicates.
# Non-login shells (tmux panes, scripts) skip this file and keep .zshenv's
# order untouched.
if [ -n "${HOMEBREW_PREFIX:-}" ]; then
	path=("${HOMEBREW_PREFIX}/bin" "${HOMEBREW_PREFIX}/sbin" $path)
fi
path=("${XDG_DATA_HOME}/mise/shims" "${HOME}/.local/bin" $path)
typeset -U path
