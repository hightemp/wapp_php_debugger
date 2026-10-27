# Single entry point for development and CI: CI runs the same targets.
# List of targets: make help

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

ROOT := $(CURDIR)
TOOLS_BIN := $(ROOT)/.tools/bin
# .exe on Windows: defined before any rule whose target uses it
EXE := $(shell go env GOEXE)
DIST := $(ROOT)/dist

VERSION ?= $(shell git describe --tags --match 'v[0-9]*' --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG := github.com/hightemp/wapp_php_debugger/server/internal/version
LDFLAGS := -s -w -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).Date=$(BUILD_DATE)

GOLANGCI_LINT_VERSION := v2.15.0
ACTIONLINT_VERSION := v1.7.12
GOVULNCHECK_VERSION := v1.9.0
OASDIFF_VERSION := v1.33.0
# The generator runs via npx: it has a TypeScript 5 peer dependency while the UI uses TypeScript 6
OPENAPI_TS_VERSION := 7.13.0
GENERATED := server/internal/api/openapi.gen.go server/internal/libclient/client.gen.go ui/src/lib/api/schema.gen.d.ts
GORELEASER_VERSION := v2.18.3
GOLANGCI_LINT := $(TOOLS_BIN)/golangci-lint
GORELEASER := $(TOOLS_BIN)/goreleaser
ACTIONLINT := $(TOOLS_BIN)/actionlint
GOVULNCHECK := $(TOOLS_BIN)/govulncheck
OASDIFF := $(TOOLS_BIN)/oasdiff
# The Wails CLI of the version in server/go.mod: AppImage, .deb, Windows resources
WAILS_VERSION = $(shell cd server && go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)
WAILS3 = $(TOOLS_BIN)/wails3$(EXE)

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
tools: $(GOLANGCI_LINT) $(ACTIONLINT) $(GOVULNCHECK) $(OASDIFF) ## Install pinned linter versions into .tools/bin

$(GOLANGCI_LINT):
	GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(ACTIONLINT):
	GOBIN=$(TOOLS_BIN) go install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

$(GOVULNCHECK):
	GOBIN=$(TOOLS_BIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

$(OASDIFF):
	GOBIN=$(TOOLS_BIN) go install github.com/oasdiff/oasdiff@$(OASDIFF_VERSION)

$(WAILS3):
	GOBIN=$(TOOLS_BIN) go install -tags "$(if $(filter gtk3,$(LINUX_GTK)),gtk3)" github.com/wailsapp/wails/v3/cmd/wails3@$(WAILS_VERSION)

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
lint: php-lint server-lint ui-lint workflows-lint spec-compat ## Run all linters

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

.PHONY: php-bundle
php-bundle: ## One-file build of the PHP library: dist/wapp-debugger.php
	cd client-php && php tools/bin/bundle src $(DIST)/wapp-debugger.php

.PHONY: php-bundle-test
php-bundle-test: php-bundle ## The PHP library tests against the one-file build
	@# auto_prepend_file: the bundle is declared before PHPUnit loads Composer's autoloader (and src/)
	cd client-php && php -d auto_prepend_file=$(DIST)/wapp-debugger.php vendor/bin/phpunit -c phpunit.bundle.xml

.PHONY: php74-test
php74-test: ## PHP library tests on PHP 7.4 in Docker, dependencies resolved for 7.4 as in CI
	rm -rf $(DIST)/php74 && mkdir -p $(DIST)/php74
	tar -C $(ROOT) --exclude=client-php/vendor --exclude=client-php/composer.lock -cf - client-php | tar -C $(DIST)/php74 -xf -
	@# the host proxy settings, when there are any, so composer downloads the same way as on the host
	docker run --rm -u $$(id -u):$$(id -g) -e COMPOSER_HOME=/tmp -e HTTP_PROXY -e HTTPS_PROXY -e NO_PROXY -e http_proxy -e https_proxy -e no_proxy \
		-v $(DIST)/php74/client-php:/app -w /app composer:2 \
		sh -c 'composer config platform.php 7.4.33 && { composer update --no-interaction --no-progress || composer update --no-interaction --no-progress; }'
	docker run --rm -u $$(id -u):$$(id -g) -v $(DIST)/php74:/work -v $(ROOT)/spec:/work/spec:ro -w /work/client-php \
		php:7.4-cli vendor/bin/phpunit

# --- Go server (server) -----------------------------------------------------

.PHONY: server-deps
server-deps: ## Go server dependencies
	cd server && go mod download

.PHONY: server-lint
server-lint: $(GOLANGCI_LINT) $(GOVULNCHECK) ## go vet, golangci-lint and govulncheck (known vulnerabilities in code that is called)
	cd server && go vet ./...
	cd server && $(GOLANGCI_LINT) run ./...
	cd server && $(GOVULNCHECK) ./...

# Each fuzz test for FUZZTIME: what a log or a remote server sends must never crash the parsers.
# A failing input lands in testdata/fuzz of the package; commit it as a regression test.
FUZZTIME ?= 10s
FUZZ_TESTS := ./internal/logformat:FuzzParseV2Chunk ./internal/logformat:FuzzParseV1 \
	./internal/logformat:FuzzDetectFormat ./internal/thirdparty:FuzzConvert

# The APIs are frozen at v1.0 (spec/frozen/README.md): any change must stay backward compatible.
# Log format v2 is held by the tests of server/internal/spec.
FROZEN := spec/frozen/v1.0

.PHONY: spec-compat
spec-compat: $(OASDIFF) ## The viewer API, the receiver and the library HTTP mode stay compatible with v1.0
	@for api in openapi ingest library-http; do \
		echo "$$api.yaml against $(FROZEN)"; \
		$(OASDIFF) breaking --fail-on WARN $(FROZEN)/$$api.yaml spec/$$api.yaml; \
	done

.PHONY: server-fuzz
server-fuzz: ## Short fuzz runs of the log parsers (FUZZTIME=10s each)
	cd server && for test in $(FUZZ_TESTS); do \
		go test "$${test%%:*}" -run '^$$' -fuzz "^$${test##*:}$$" -fuzztime $(FUZZTIME); \
	done

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
DESKTOP_BIN := $(DIST)/wapp-debugger-desktop$(EXE)

.PHONY: desktop-deps
desktop-deps: ## Check the C compiler and, on Linux, GTK and WebKitGTK; prints how to install what is missing
	scripts/desktop-deps.sh $(LINUX_GTK)

.PHONY: desktop-build
desktop-build: desktop-deps ui-build ## Build the desktop app for the current OS (requires CGO)
	cd server && CGO_ENABLED=1 go build -tags "$(DESKTOP_TAGS)" -trimpath -ldflags "$(LDFLAGS)" -o $(DESKTOP_BIN) ./cmd/wapp-debugger-desktop

.PHONY: icons
icons: ## Render desktop/icons/icon.svg into the PNG, ICO and ICNS icons (needs ui deps and Playwright's Chromium)
	node scripts/icons.mjs

.PHONY: desktop-vet
desktop-vet: desktop-deps ui-build ## go vet the desktop code (with its build tags; the embedded UI must be built)
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
ui-lint: ## ESLint, Stylelint (colors only from theme tokens), Prettier, svelte-check, npm audit
	cd ui && npm run lint
	cd ui && npm run check
	# What ships in the viewer: dev tools (linters, test runners) are not in the build
	cd ui && npm audit --omit=dev --audit-level=low

.PHONY: ui-test
ui-test: ## UI tests (Vitest)
	cd ui && npm test

.PHONY: e2e
e2e: build ## Browser tests (Playwright) of the built viewer on the v2 fixtures
	cd ui && NO_PROXY='*' no_proxy='*' npx playwright test --project chromium

# Screens are compared with ui/e2e/__screenshots__ inside the Playwright image: the same fonts on
# every machine and in CI. The image matches @playwright/test in ui/package.json.
PLAYWRIGHT_IMAGE := mcr.microsoft.com/playwright:v1.63.0-noble
VISUAL_FLAGS ?=

.PHONY: e2e-visual
e2e-visual: ## Screenshots of the built viewer in every theme against the committed ones (Docker)
	@test -x $(DIST)/wapp-debugger || $(MAKE) build
	docker run --rm --network host --user "$$(id -u):$$(id -g)" -e HOME=/tmp -e CI \
		-v "$(ROOT):/work" -w /work/ui $(PLAYWRIGHT_IMAGE) \
		npx playwright test --project visual $(VISUAL_FLAGS)

.PHONY: e2e-visual-update
e2e-visual-update: ## Take the screenshots again after a change of the look that is meant
	$(MAKE) e2e-visual VISUAL_FLAGS=--update-snapshots

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

.PHONY: release
release: $(GORELEASER) ui-build php-bundle ## Publish a draft GitHub release for the current tag (CI only: needs GITHUB_TOKEN)
	scripts/release-check.sh "$$(git describe --tags --exact-match --match 'v[0-9]*')"
	$(GORELEASER) release --clean

.PHONY: release-snapshot
release-snapshot: $(GORELEASER) ui-build php-bundle ## Local release check: server archives for all OSes in dist/release
	$(GORELEASER) release --snapshot --clean $(if $(shell command -v syft),,--skip=sbom)

.PHONY: desktop-package
desktop-package: $(WAILS3) ## Desktop app and its installers for the current OS into dist/release-desktop (unsigned)
	scripts/desktop-package.sh $(VERSION) $(WAILS3)

.PHONY: docker-build
docker-build: ## Build the server image wapp-debugger:dev (see Dockerfile)
	docker build -t wapp-debugger:dev --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) .

# --- Misc -------------------------------------------------------------------

.PHONY: install-test
install-test: ## Test scripts/install.sh against a fake GitHub Releases server
	scripts/install_test.sh

.PHONY: changed-areas-test
changed-areas-test: ## Test scripts/changed-areas.sh, which CI uses to skip jobs a change does not need
	scripts/changed-areas_test.sh

.PHONY: release-check-test
release-check-test: ## Test scripts/release-check.sh: no stable release with a pre-release library
	scripts/release-check_test.sh

.PHONY: desktop-deps-test
desktop-deps-test: ## Test scripts/desktop-deps.sh against fake pkg-config and compilers
	scripts/desktop-deps_test.sh

.PHONY: ssh-e2e
ssh-e2e: ## The viewer against a real OpenSSH server in Docker (throwaway key, no ssh-agent)
	scripts/ssh-e2e.sh

.PHONY: scripts-lint
scripts-lint: ## shellcheck the shell scripts (shellcheck binary, or its Docker image)
	@if command -v shellcheck >/dev/null 2>&1; then shellcheck scripts/*.sh; \
	else docker run --rm -v "$(ROOT)/scripts:/s:ro" koalaman/shellcheck:v0.10.0 /s/install.sh /s/install_test.sh /s/ssh-e2e.sh /s/desktop-package.sh /s/changed-areas.sh /s/changed-areas_test.sh /s/release-check.sh /s/release-check_test.sh /s/desktop-deps.sh /s/desktop-deps_test.sh; fi

.PHONY: workflows-lint
workflows-lint: $(ACTIONLINT) ## Lint GitHub Actions workflows
	$(ACTIONLINT)

.PHONY: fixtures-v1
fixtures-v1: ## Regenerate v1 log fixtures (requires Docker)
	spec/fixtures/v1/generator/generate.sh

.PHONY: fixtures-v2-library
fixtures-v2-library: ## Regenerate the logs of the real PHP library 2.x (requires Docker)
	spec/fixtures/v2-library/generator/generate.sh
