# Единая точка входа для разработки и CI (п. 3.1 плана). CI вызывает те же цели.
# Список целей: make help

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
# Генератор запускается через npx: его peer-зависимость — TypeScript 5, а UI на TypeScript 6
OPENAPI_TS_VERSION := 7.13.0
GENERATED := server/internal/api/openapi.gen.go server/internal/libclient/client.gen.go ui/src/lib/api/schema.gen.d.ts
GORELEASER_VERSION := v2.18.3
GOLANGCI_LINT := $(TOOLS_BIN)/golangci-lint
GORELEASER := $(TOOLS_BIN)/goreleaser
ACTIONLINT := $(TOOLS_BIN)/actionlint

.PHONY: help
help: ## Показать список целей
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: doctor
doctor: ## Проверить, что нужные инструменты установлены
	@status=0; \
	check() { if command -v "$$1" >/dev/null 2>&1; then printf "  ok      %-10s %s\n" "$$1" "$$($$2 2>&1 | head -1)"; else printf "  MISSING %-10s %s\n" "$$1" "$$3"; status=1; fi; }; \
	check go "go -C server version" "https://go.dev/dl/ (Go 1.21+; версию 1.26 из server/go.mod он скачает сам)"; \
	check node "node --version" "https://nodejs.org/ (нужен Node 22+)"; \
	check npm "npm --version" "ставится вместе с Node"; \
	check php "php --version" "нужен PHP 7.4+"; \
	check composer "composer --version" "https://getcomposer.org/"; \
	check docker "docker --version" "нужен только для make fixtures-v1 и тестов на PHP 7.4"; \
	for tool in $(GOLANGCI_LINT) $(ACTIONLINT); do \
		if [ -x "$$tool" ]; then printf "  ok      %-10s %s\n" "$$(basename $$tool)" "$$tool"; else printf "  MISSING %-10s выполните: make tools\n" "$$(basename $$tool)"; status=1; fi; \
	done; \
	exit $$status

.PHONY: tools
tools: $(GOLANGCI_LINT) $(ACTIONLINT) ## Установить линтеры закреплённых версий в .tools/bin

$(GOLANGCI_LINT):
	GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(ACTIONLINT):
	GOBIN=$(TOOLS_BIN) go install github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

$(GORELEASER):
	GOBIN=$(TOOLS_BIN) go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

# --- Общие цели -------------------------------------------------------------

.PHONY: deps
deps: php-deps server-deps ui-deps ## Установить зависимости всех компонентов

.PHONY: lint
lint: php-lint server-lint ui-lint workflows-lint ## Все линтеры

.PHONY: test
test: php-test server-test ui-test ## Все тесты

.PHONY: build
build: server-build ## Собрать бинарник wapp-debugger в dist/

.PHONY: ci
ci: deps lint test build ## Всё, что делает CI, одной командой

.PHONY: fmt
fmt: ## Отформатировать код всех компонентов
	cd client-php && vendor/bin/php-cs-fixer fix
	cd server && gofmt -w . && $(GOLANGCI_LINT) fmt ./...
	cd ui && npm run format

.PHONY: clean
clean: ## Удалить артефакты сборки
	rm -rf $(DIST) server/web/dist

# --- PHP-библиотека (client-php) --------------------------------------------

.PHONY: php-deps
php-deps: ## Зависимости PHP-библиотеки
	cd client-php && composer install --no-interaction --no-progress

.PHONY: php-lint
php-lint: ## Code style, PHPStan и проверка имён (без венгерской нотации)
	cd client-php && vendor/bin/php-cs-fixer check --diff
	cd client-php && vendor/bin/phpstan analyse --no-progress
	cd client-php && php tools/bin/check-naming src tests tools

.PHONY: php-test
php-test: ## Тесты PHP-библиотеки
	cd client-php && vendor/bin/phpunit

.PHONY: php74-test
php74-test: ## Тесты PHP-библиотеки на PHP 7.4 в Docker
	docker run --rm -u $$(id -u):$$(id -g) -v $(ROOT)/client-php:/app -w /app php:7.4-cli vendor/bin/phpunit

# --- Go-сервер (server) -----------------------------------------------------

.PHONY: server-deps
server-deps: ## Зависимости Go-сервера
	cd server && go mod download

.PHONY: server-lint
server-lint: $(GOLANGCI_LINT) ## go vet и golangci-lint
	cd server && go vet ./...
	cd server && $(GOLANGCI_LINT) run ./...

.PHONY: server-test
server-test: ## Тесты Go-сервера (с детектором гонок, без кэша: тесты читают spec/ вне модуля)
	cd server && go test -race -count=1 ./...

.PHONY: server-build
server-build: ui-build ## Собрать бинарник со встроенным UI
	cd server && CGO_ENABLED=0 go build -tags embedui -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/wapp-debugger ./cmd/wapp-debugger
	@$(DIST)/wapp-debugger version

# --- Десктоп (Wails v3) ------------------------------------------------------

# На Linux Wails v3 по умолчанию собирается с GTK4 + WebKitGTK 6.0; для GTK3 + WebKit2GTK 4.1: make desktop-build LINUX_GTK=gtk3
LINUX_GTK ?= gtk4
DESKTOP_TAGS := desktop embedui $(if $(filter gtk3,$(LINUX_GTK)),gtk3)
EXE := $(shell go env GOEXE)
DESKTOP_BIN := $(DIST)/wapp-debugger-desktop$(EXE)

.PHONY: desktop-build
desktop-build: ui-build ## Собрать десктоп-приложение под текущую ОС (нужен CGO)
	cd server && CGO_ENABLED=1 go build -tags "$(DESKTOP_TAGS)" -trimpath -ldflags "$(LDFLAGS)" -o $(DESKTOP_BIN) ./cmd/wapp-debugger-desktop

.PHONY: desktop-vet
desktop-vet: ## go vet кода десктопа (с тегами сборки)
	cd server && CGO_ENABLED=1 go vet -tags "$(DESKTOP_TAGS)" ./cmd/wapp-debugger-desktop

.PHONY: desktop-selftest
desktop-selftest: desktop-build ## Открыть окно и проверить REST и поток событий внутри Wails (п. 10.27)
	@if [ "$$(go env GOOS)" = linux ] && [ -z "$${DISPLAY:-}" ]; then \
		xvfb-run -a -s "-screen 0 1280x800x24" $(DESKTOP_BIN) --self-test; \
	else \
		$(DESKTOP_BIN) --self-test; \
	fi

.PHONY: desktop-dev
desktop-dev: desktop-build ## Запустить десктоп (LOGS_DIR=… — папка логов проекта)
	$(DESKTOP_BIN) $(if $(LOGS_DIR),--logs-dir $(LOGS_DIR))

# --- Веб-интерфейс (ui) -----------------------------------------------------

.PHONY: ui-deps
ui-deps: ## Зависимости UI
	cd ui && npm ci --no-audit --no-fund

.PHONY: ui-lint
ui-lint: ## ESLint, Prettier и svelte-check
	cd ui && npm run lint
	cd ui && npm run check

.PHONY: ui-test
ui-test: ## Тесты UI (Vitest)
	cd ui && npm test

.PHONY: ui-build
ui-build: ## Собрать UI в server/web/dist
	cd ui && npm run build

.PHONY: ui-dev
ui-dev: ## Vite dev server
	cd ui && npm run dev

# --- Кодогенерация из spec/ -------------------------------------------------

.PHONY: generate
generate: ## Сгенерировать Go- и TS-код из spec/*.yaml
	cd server && go generate ./...
	cd ui && npx -y openapi-typescript@$(OPENAPI_TS_VERSION) ../spec/openapi.yaml -o src/lib/api/schema.gen.d.ts

.PHONY: generate-check
generate-check: generate ## Проверить, что сгенерированный код совпадает с закоммиченным
	@if [ -n "$$(git status --porcelain -- $(GENERATED))" ]; then \
		echo "Сгенерированный код устарел: выполните make generate и закоммитьте изменения"; \
		git status --short -- $(GENERATED); git --no-pager diff --stat -- $(GENERATED); exit 1; \
	fi

# --- Релиз -------------------------------------------------------------------

.PHONY: release-snapshot
release-snapshot: $(GORELEASER) ui-build ## Локальная проверка релиза: архивы сервера для всех ОС в dist/release
	$(GORELEASER) release --snapshot --clean

# --- Прочее -----------------------------------------------------------------

.PHONY: workflows-lint
workflows-lint: $(ACTIONLINT) ## Проверить GitHub Actions workflow
	$(ACTIONLINT)

.PHONY: fixtures-v1
fixtures-v1: ## Перегенерировать фикстуры логов v1 (нужен Docker)
	spec/fixtures/v1/generator/generate.sh
