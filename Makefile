# one -ignore flag per non-empty line in .licenseignore (quoted to avoid shell globbing)
LICENSE_HOLDER := 'Bitwise Media Group Ltd.'
LICENSE_IGNORE := $(foreach pattern,$(shell cat .licenseignore 2>/dev/null),-ignore '$(pattern)')

# Developer tasks. `make help` lists targets; `make pr` is the full local gate.
APP     := dotty
APP_PKG := ./cmd
MODULE  := $(shell go list -m)

# Version metadata stamped into the binary via -ldflags. GoReleaser injects the
# same vars at the same import path ($(MODULE)/internal/version) on tagged releases.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.BuildDate=$(DATE)

# Fuzzing: `make fuzz` runs one target (FUZZ=) for FUZZTIME. `go test -fuzz`
# accepts a single package only, so FUZZ_PKG must name one package.
FUZZ_PKG ?= ./internal/cli
FUZZ     ?= FuzzExtractFlags
FUZZTIME ?= 20s

# Run the Node lint/format CLIs straight from node_modules so the versions pinned
# in package.json / package-lock.json are used — never a global or npx copy.
NPMBIN := ./node_modules/.bin

# Go developer CLIs (addlicense, golangci-lint, goreleaser, govulncheck, syft,
# gotestsum, gocover-cobertura) are pinned in tools/go.mod — a separate module
# so their dependency graphs never touch the application's go.mod —
# and invoked with `go tool -modfile=tools/go.mod <name>`: compiled into the build
# cache on first use, no GOBIN, no binaries to manage. -modfile anchors on the root
# go.mod and runs the tool in the current directory, so relative paths just work.
# Do not add a go.work that `use`s tools/ — -modfile cannot be used in workspace mode.

.DEFAULT_GOAL := help

.PHONY: help
help: ## List available targets
	@ grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

.PHONY: pr
pr: tidy license fmt lint test fuzz build docs snapshot ## full local gate for pull request

.PHONY: ci
ci: lint test fuzz build docs snapshot ## full ci gate

# Install the pinned Node tools exactly as locked in package-lock.json.
# Re-runs only when package.json / the lockfile change.
node_modules: package.json package-lock.json
	@ npm ci --ignore-scripts --no-fund
	@ touch node_modules

.PHONY: fmt
fmt: node_modules ## format the go codebase
	@ go fmt ./...
	@ go tool -modfile=tools/go.mod golangci-lint run --fix
	@ npm run lint:fix
	@ npm run format

.PHONY: tidy
tidy: ## tidy go mod/sum
	@ rm -f go.sum; go mod tidy

.PHONY: lint
lint: node_modules ## lint the go code, prose, and config (check mode)
	@ go tool -modfile=tools/go.mod addlicense -l mit -c $(LICENSE_HOLDER) -s=only $(LICENSE_IGNORE) -check .
	@ go tool -modfile=tools/go.mod golangci-lint run
	@ go tool -modfile=tools/go.mod govulncheck ./...
	@ npm run lint
	@ npm run format:check

.PHONY: license
license: ## inject SPDX license headers (addlicense, pinned in tools/go.mod)
	@ go tool -modfile=tools/go.mod addlicense -l mit -c $(LICENSE_HOLDER) -s=only $(LICENSE_IGNORE) .

# -covermode=atomic is the race-safe counter mode `-race` requires. gotestsum runs
# the suite, streams human-readable output, and writes a JUnit report in one pass
# (propagating the test exit code, which a bare `go test | …` pipe would swallow);
# gocover-cobertura turns the profile into Cobertura XML. Both are pinned in
# tools/go.mod and run via `go tool`. Coverage (cobertura-coverage.xml) and test
# results (junit.xml) land in coverage/ where the reusable CI workflow uploads them
# to Codecov.
.PHONY: test
test: ## run the unit tests with coverage (+ fuzz seed corpora)
	@ mkdir -p coverage
	@ go tool -modfile=tools/go.mod gotestsum --junitfile coverage/junit.xml -- \
		-race -covermode=atomic -coverprofile=coverage/coverage.out ./...
	@ go tool -modfile=tools/go.mod gocover-cobertura <coverage/coverage.out >coverage/cobertura-coverage.xml

.PHONY: fuzz
fuzz: ## fuzz one target (FUZZ=FuzzExtractFlags FUZZTIME=20s FUZZ_PKG=./internal/cli)
	@ go test -run '^$$' -fuzz '^$(FUZZ)$$' -fuzztime $(FUZZTIME) $(FUZZ_PKG)

.PHONY: build
build: ## build the binary (./$(APP)) with version ldflags
	@ CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(APP) $(APP_PKG)

.PHONY: link
link: build ## links the local build
	@ ln -fs $(CURDIR)/$(APP) /usr/local/bin
	@ mkdir -p ~/.local/share/dotty
	@ ln -fs $(CURDIR)/$(APP) ~/.local/share/dotty/dotty-ssh-askpass

.PHONY: run
run: build ## build and run locally (override args via ARGS=...)
	./$(APP) $(ARGS)

.PHONY: docs
docs: build ## regenerate the CLI reference (docs/cli) from the cobra command tree
	@ ./$(APP) docs --out docs/cli --format markdown
	@ ./$(APP) docs --out docs/man --format man
	@ uv run zensical build

# Docs site (Zensical). Kept out of `pr`/`ci` so the gate needs no Python; run
# these directly. `uv` provisions Python + zensical from pyproject.toml on first
# use. The built site/ is git-ignored.
.PHONY: serve
serve: ## serve the docs site locally (zensical)
	@ uv run zensical serve

# --skip=sign: cosign keyless signing needs the GitHub Actions OIDC token, so
# it only works in the release workflow — locally it would fail or prompt.
.PHONY: snapshot
snapshot: ## build local release snapshot (binaries + archives, no publish or signing)
	@ go tool -modfile=tools/go.mod goreleaser release --snapshot --clean --skip=sign

.PHONY: release
release: ## build and publish a release (needs a vX.Y.Z tag + creds)
	@ go tool -modfile=tools/go.mod goreleaser release --clean
