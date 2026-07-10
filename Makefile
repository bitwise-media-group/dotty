# dotty — utilities for a terminal-driven workflow and dotfiles.
#
# Everything lives in mise tasks: the go-cli archetype (build/test/lint/release
# machinery + pinned tools) comes from the shared toolchain submodule at .mise/,
# selected in the root mise.toml, which also defines the repo-local tasks
# (docs, link, run, fuzz). This Makefile is only the thin forwarding shim —
# `make <task>` == `mise run <task>`.
include .mise/mise.mk
