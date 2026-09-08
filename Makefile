# dotty — utilities for a terminal-driven workflow and dotfiles.
#
# Everything lives in mise tasks: the go archetype (build/test/lint/release
# machinery) and the common tasks (license, prose, workflow, container, and
# shell lint) come from the shared toolchain submodule at .mise/, selected in
# the root mise.toml, which also defines the repo-local tasks (docs, link, run,
# fuzz). This Makefile is only the thin forwarding shim —
# `make <task>` == `mise run <task>`.
include .mise/archetypes/go/include.mk
