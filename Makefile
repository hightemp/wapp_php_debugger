# Single entry point for development and CI: CI runs the same targets.
# List of targets: make help

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

ROOT := $(CURDIR)
TOOLS_BIN := $(ROOT)/.tools/bin
DIST := $(ROOT)/dist

VERSION ?= $(shell git describe --tags --match 'v[0-9]*' --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG := github.com/hightemp/wapp_php_debugger/server/internal/version
LDFLAGS := -s -w -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).Date=$(BUILD_DATE)

GOLANGCI_LINT_VERSION := v2.15.0
ACTIONLINT_VERSION := v1.7.12
# The generator runs via npx: it has a TypeScript 5 peer dependency while the UI uses TypeScript 6
OPENAPI_TS_VERSION := 7.13.0
GENERATED := server/internal/api/openapi.gen.go server/internal/libclient/client.gen.go ui/src/lib/api/schema.gen.d.ts
GORELEASER_VERSION := v2.18.3
GOLANGCI_LINT := $(TOOLS_BIN)/golangci-lint
GORELEASER := $(TOOLS_BIN)/goreleaser
ACTIONLINT := $(TOOLS_BIN)/actionlint

.PHONY: help
help: ## Show the list of targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: doctor
doctor: ## Check that the required tools are installed
	@status=0; \
	check() { if command -v "$$1" >/dev/null 2>&1; then printf "  ok      %-10s %s\n" "$$1" "$$($$2 2>&1 | head -1)"; else printf "  MISSING %-10s %s\n" "$$1" "$$3"; status=1; fi; }; \
	check go "go -C server version" "https://go.dev/dl/ (Go 1.21+; it downloads 1.26 from server/go.mod by itself)"; \
	check node "node --version" "https://nodejs.org/ (Node 22+ required)"; \
	check npm "npm --version" "installed with Node"; \
	check php "php --version" "PHP 7.4+ required"; \
	check composer "composer --version" "https://getcomposer.org/"; \
	check docker "docker --version" "only needed for make fixtures-v1 and PHP 7.4 tests"; \
	for tool in $(GOLANGCI_LINT) $(ACTIONLINT); do \
		if [ -x "$$tool" ]; then printf "  ok      %-10s %s\n" "$$(basename $$tool)" "$$tool"; else printf "  MISSING %-10s run: make tools\n" "$$(basename $$tool)"; status=1; fi; \
	done; \
	exit $$status

.PHONY: tools
tools: $(GOLANGCI_LINT) $(ACTIONLINT) ## Install pinned linter versions into .tools/bin

$(GOLANGCI_LINT):
	GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(ACTIONLINT):
	GOBIN=$(TOOLS_BIN) go install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

# GoReleaser: the official binary, checked against the release checksums. Building it with go install
# needs a newer Go than the project and pulls hundreds of modules, which made CI flaky.
GORELEASER_OS := $(shell uname -s)
GORELEASER_ARCH := $(subst aarch64,arm64,$(subst amd64,x86_64,$(shell uname -m)))
GORELEASER_ARCHIVE := goreleaser_$(GORELEASER_OS)_$(GORELEASER_ARCH).tar.gz
GORELEASER_URL := https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)

$(GORELEASER):
	@mkdir -p $(TOOLS_BIN) $(TOOLS_BIN)/.goreleaser
	cd $(TOOLS_BIN)/.goreleaser && curl -fsSLO $(GORELEASER_URL)/$(GORELEASER_ARCHIVE) && curl -fsSLO $(GORELEASER_URL)/checksums.txt \
		&& grep " $(GORELEASER_ARCHIVE)$$" checksums.txt | sha256sum -c - \
		&& tar -xzf $(GORELEASER_ARCHIVE) goreleaser && mv goreleaser $(GORELEASER)
	rm -rf $(TOOLS_BIN)/.goreleaser

# --- Common targets ----------------------------------------------------------

.PHONY: deps
deps: php-deps server-deps ui-deps ## Install dependencies of all components

.PHONY: lint
lint: php-lint server-lint ui-lint workflows-lint ## Run all linters

.PHONY: test
test: php-test server-test ui-test ## Run all tests

.PHONY: build
build: server-build ## Build the wapp-debugger binary into dist/

.PHONY: ci
ci: deps lint test build ## Everything CI does, in one command

.PHONY: fmt
fmt: ## Format the code of all components
	cd client-php && vendor/bin/php-cs-fixer fix
	cd server && gofmt -w . && $(GOLANGCI_LINT) fmt ./...
	cd ui && npm run format

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(DIST) server/web/dist

# --- PHP library (client-php) -----------------------------------------------

.PHONY: php-deps
php-deps: ## PHP library dependencies
	cd client-php && composer install --no-interaction --no-progress

.PHONY: php-lint
php-lint: ## Code style, PHPStan and the naming check (no Hungarian notation)
	cd client-php && vendor/bin/php-cs-fixer check --diff
	cd client-php && vendor/bin/phpstan analyse --no-progress
	cd client-php && php tools/bin/check-naming src tests tools

.PHONY: php-test
php-test: ## PHP library tests
	cd client-php && vendor/bin/phpunit

.PHONY: php74-test
php74-test: ## PHP library tests on PHP 7.4 in Docker
	docker run --rm -u $$(id -u):$$(id -g) -v $(ROOT)/client-php:/app -w /app php:7.4-cli vendor/bin/phpunit

# --- Go server (server) -----------------------------------------------------

.PHONY: server-deps
server-deps: ## Go server dependencies
	cd server && go mod download

.PHONY: server-lint
server-lint: $(GOLANGCI_LINT) ## go vet and golangci-lint
	cd server && go vet ./...
	cd server && $(GOLANGCI_LINT) run ./...

.PHONY: server-test
server-test: ## Go server tests (race detector, no cache: tests read spec/ outside the module)
	cd server && go test -race -count=1 ./...

.PHONY: server-build
server-build: ui-build ## Build the binary with the embedded UI
	cd server && CGO_ENABLED=0 go build -tags embedui -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/wapp-debugger ./cmd/wapp-debugger
	@$(DIST)/wapp-debugger version

# --- Desktop (Wails v3) -----------------------------------------------------

# On Linux Wails v3 builds with GTK4 + WebKitGTK 6.0 by default; for GTK3 + WebKit2GTK 4.1: make desktop-build LINUX_GTK=gtk3
LINUX_GTK ?= gtk4
DESKTOP_TAGS := desktop embedui $(if $(filter gtk3,$(LINUX_GTK)),gtk3)
EXE := $(shell go env GOEXE)
DESKTOP_BIN := $(DIST)/wapp-debugger-desktop$(EXE)

.PHONY: desktop-build
desktop-build: ui-build ## Build the desktop app for the current OS (requires CGO)
	cd server && CGO_ENABLED=1 go build -tags "$(DESKTOP_TAGS)" -trimpath -ldflags "$(LDFLAGS)" -o $(DESKTOP_BIN) ./cmd/wapp-debugger-desktop

.PHONY: desktop-vet
desktop-vet: ui-build ## go vet the desktop code (with its build tags; the embedded UI must be built)
	cd server && CGO_ENABLED=1 go vet -tags "$(DESKTOP_TAGS)" ./cmd/wapp-debugger-desktop

.PHONY: desktop-selftest
desktop-selftest: desktop-build ## Open a window and check REST and the event stream inside Wails
	@if [ "$$(go env GOOS)" = linux ] && [ -z "$${DISPLAY:-}" ]; then \
		GSK_RENDERER=cairo WEBKIT_DISABLE_DMABUF_RENDERER=1 LIBGL_ALWAYS_SOFTWARE=1 \
			xvfb-run -a -s "-screen 0 1280x800x24" $(DESKTOP_BIN) --self-test; \
	else \
		$(DESKTOP_BIN) --self-test; \
	fi

.PHONY: desktop-dev
desktop-dev: desktop-build ## Run the desktop app (LOGS_DIR=… is the project logs directory)
	$(DESKTOP_BIN) $(if $(LOGS_DIR),--logs-dir $(LOGS_DIR))

# --- Web UI (ui) ------------------------------------------------------------

.PHONY: ui-deps
ui-deps: ## UI dependencies
	cd ui && npm ci --no-audit --no-fund

.PHONY: ui-lint
ui-lint: ## ESLint, Prettier and svelte-check
	cd ui && npm run lint
	cd ui && npm run check

.PHONY: ui-test
ui-test: ## UI tests (Vitest)
	cd ui && npm test

.PHONY: ui-build
ui-build: ## Build the UI into server/web/dist
	cd ui && npm run build

.PHONY: ui-dev
ui-dev: ## Vite dev server
	cd ui && npm run dev

# --- Code generation from spec/ ---------------------------------------------

.PHONY: generate
generate: ## Generate Go and TS code from spec/*.yaml
	cd server && go generate ./...
	cd ui && npx -y openapi-typescript@$(OPENAPI_TS_VERSION) ../spec/openapi.yaml -o src/lib/api/schema.gen.d.ts

.PHONY: generate-check
generate-check: generate ## Check that generated code matches what is committed
	@if [ -n "$$(git status --porcelain -- $(GENERATED))" ]; then \
		echo "Generated code is out of date: run make generate and commit the changes"; \
		git status --short -- $(GENERATED); git --no-pager diff --stat -- $(GENERATED); exit 1; \
	fi

# --- Release ----------------------------------------------------------------

.PHONY: release-snapshot
release-snapshot: $(GORELEASER) ui-build ## Local release check: server archives for all OSes in dist/release
	$(GORELEASER) release --snapshot --clean

# --- Misc -------------------------------------------------------------------

.PHONY: workflows-lint
workflows-lint: $(ACTIONLINT) ## Lint GitHub Actions workflows
	$(ACTIONLINT)

.PHONY: fixtures-v1
fixtures-v1: ## Regenerate v1 log fixtures (requires Docker)
	spec/fixtures/v1/generator/generate.sh
