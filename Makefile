# Makefile for the seal CLI.
#

# --- Variables ----------------------------------------------------

# BIN is the local output path for `make build`. Lives under bin/
# (gitignored) so a stray binary in the repo root can't get
# committed accidentally.
BIN ?= bin/seal

# VERSION is stamped into the binary via -ldflags. Defaults to a
# best-effort `git describe` so local builds report something more
# useful than "dev"; release builds override via the release
# workflow's TAG env.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# GO is the toolchain entry point. Overridable so a developer can
# point at a specific toolchain (e.g. /usr/local/go-1.26/bin/go).
GO ?= go

# LDFLAGS mirrors the release workflow's flags so local builds
# produce binaries close to what gets shipped. -s/-w strip the
# symbol + DWARF tables; -X stamps the version. -buildid= is
# deferred to the release-snapshot target because reproducibility
# only matters for shipped artifacts.
LDFLAGS := -X main.Version=$(VERSION)

# --- Default goal -------------------------------------------------

# Make `make` alone print help instead of trying to build the first
# target it sees. Friendlier for newcomers.
.DEFAULT_GOAL := help

# --- Help ---------------------------------------------------------

.PHONY: help
help: ## Print this help message
	@# Scan this file for `target: ... ## comment` lines and print
	@# them in a column-aligned table. Each target documents itself
	@# in its `##` suffix; this `awk` script is the universal Make
	@# idiom for self-documenting Makefiles.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / { \
		printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 \
	}' $(MAKEFILE_LIST)

# --- Build / install ----------------------------------------------

.PHONY: build
build: ## Build the seal binary into bin/seal (stamped with VERSION)
	@mkdir -p $(dir $(BIN))
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/seal

.PHONY: install
install: ## Install seal into $GOBIN (or $GOPATH/bin)
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/seal

# --- Tests --------------------------------------------------------

.PHONY: test
test: ## Run unit + E2E tests (single CPU run)
	$(GO) test ./...

.PHONY: test-race
test-race: ## Run tests with the race detector and cache disabled (matches CI)
	$(GO) test -race -count=1 ./...

.PHONY: test-unit
test-unit: ## Run unit tests only (skip the slower E2E suite)
	$(GO) test ./internal/...

.PHONY: test-e2e
test-e2e: ## Run only the E2E tests (builds the binary first)
	$(GO) test ./cmd/seal/...

.PHONY: cover
cover: ## Generate an HTML coverage report at /tmp/seal-cover.html
	@# Two-step because `go test -coverprofile` and `go tool cover
	@# -html` are separate commands; the pipe-to-tmp pattern keeps
	@# the intermediate file outside the source tree.
	$(GO) test -coverprofile=/tmp/seal-cover.out ./...
	$(GO) tool cover -html=/tmp/seal-cover.out -o /tmp/seal-cover.html
	@echo "coverage report: /tmp/seal-cover.html"

# --- Static checks ------------------------------------------------

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Apply gofmt to all sources (mutates tree)
	@# gofmt -l prints any unformatted files; -w writes the
	@# canonical form back. We pass both flags so the developer
	@# sees what changed before it gets rewritten.
	gofmt -l -w .

.PHONY: fmt-check
fmt-check: ## Fail if any source files are not gofmt-canonical
	@# `gofmt -l` exits 0 even when it lists files. We grep for any
	@# output and exit 1 ourselves so this target works in CI gates.
	@out=$$(gofmt -l . 2>&1); \
	if [ -n "$$out" ]; then \
		echo "::error::these files are not gofmt-clean:"; \
		echo "$$out"; \
		exit 1; \
	fi

.PHONY: tidy
tidy: ## Run go mod tidy (mutates go.mod / go.sum)
	$(GO) mod tidy

.PHONY: tidy-check
tidy-check: ## Fail if go mod tidy would change go.mod or go.sum
	@# Same idea as the CI workflow's tidy step: run tidy, then
	@# diff. Failing here in `make check` catches stale deps before
	@# CI does.
	$(GO) mod tidy
	@if ! git diff --exit-code go.mod go.sum >/dev/null; then \
		echo "::error::go.mod/go.sum out of date - run 'make tidy'"; \
		exit 1; \
	fi

# --- Aggregate / CI parity ----------------------------------------

.PHONY: check
check: fmt-check vet tidy-check test-race ## Run everything CI runs (no mutation)
	@echo "all checks passed"

# --- Cross-compile (smoke-test the release workflow locally) -------

.PHONY: release-snapshot
release-snapshot: ## Build all 5 release targets into bin/ (mirrors release.yml flags)
	@# Same flag set as .github/workflows/release.yml so local
	@# smoke-tests catch flag-related regressions before tagging.
	@# Loop is unrolled (5 explicit invocations) rather than a
	@# Make foreach so each variant gets its own line in the log,
	@# making it obvious which target failed if one breaks.
	@mkdir -p bin
	GOOS=linux   GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w -buildid= $(LDFLAGS)" -o bin/seal-linux-amd64       ./cmd/seal
	GOOS=linux   GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w -buildid= $(LDFLAGS)" -o bin/seal-linux-arm64       ./cmd/seal
	GOOS=darwin  GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w -buildid= $(LDFLAGS)" -o bin/seal-darwin-amd64      ./cmd/seal
	GOOS=darwin  GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w -buildid= $(LDFLAGS)" -o bin/seal-darwin-arm64      ./cmd/seal
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w -buildid= $(LDFLAGS)" -o bin/seal-windows-amd64.exe ./cmd/seal
	@ls -lh bin/seal-*

# --- Clean --------------------------------------------------------

.PHONY: clean
clean: ## Remove built binaries and coverage artifacts
	rm -rf bin/
	rm -f /tmp/seal-cover.out /tmp/seal-cover.html
