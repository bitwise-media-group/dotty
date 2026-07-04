# dotty — utilities for a terminal-driven workflow and dotfiles.
#
# The common Go build/lint/test/release machinery lives in the shared Makefile
# library (bitwise-media-group/make), consumed as the `make/` submodule and
# included below. Only dotty's repo-specific knobs and long-tail targets live
# here; the canonical lint/build/test/ci/pr contract comes from go-cli.mk.
APP     := dotty
APP_PKG := ./cmd

# `go test -fuzz` accepts a single package, so point fuzz at one target/package.
FUZZ_PKG := ./internal/cli
FUZZ     := FuzzExtractFlags

include make/go-cli.mk

# ---- repo-local targets (the long tail the library intentionally omits) ------

.PHONY: docs link run
# Regenerate the CLI reference from the cobra command tree, then render the site.
# (Kept repo-local: generating a CLI reference is app-specific. `serve` — and the
# zensical plumbing behind it — come from the library's docs.mk.)
docs: build ## regenerate the CLI reference (docs/cli, docs/man) and build the site
	@ ./$(APP) docs --out docs/cli --format markdown
	@ ./$(APP) docs --out docs/man --format man
	@ uv run zensical build

link: build ## symlink the local build into PATH (+ the ssh-askpass helper)
	@ ln -fs $(CURDIR)/$(APP) /usr/local/bin
	@ mkdir -p ~/.local/share/dotty
	@ ln -fs $(CURDIR)/$(APP) ~/.local/share/dotty/dotty-ssh-askpass

run: build ## build and run locally (override args via ARGS=...)
	./$(APP) $(ARGS)
