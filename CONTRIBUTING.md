# Contributing

The plan and all decisions are in [IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md) (in Russian): section 8 is the order of work, section 10 the decisions.

## Language

The project is open source and **English is the base language**: code, comments, identifiers, commit messages, CLI output, API error messages, specifications and documentation. Russian is the second language: the UI is translated (`ui/src/lib/i18n`) and user documentation has a Russian version (`README.ru.md`). A change to user-facing docs updates both versions.

## Repository layout

| Directory | What | Language |
|---|---|---|
| `client-php/` | PHP library (the debugger), Composer package, PHP 7.4+; `v1/` is the current library, urgent fixes only, outside the linters | PHP |
| `server/` | Viewer: CLI, server, log import, desktop entry point | Go 1.26+ |
| `ui/` | Web UI, embedded into the Go binary | Svelte 5 + TypeScript |
| `desktop/` | Notes on the Wails desktop build | — |
| `spec/` | Contracts: fixtures, JSON Schema, OpenAPI | — |

## Commands

Every check runs through `make`; CI calls the same targets:

```sh
make doctor   # what is installed, what is missing
make tools    # pinned linter versions in .tools/bin
make deps     # dependencies of all components
make test     # all tests
make lint     # all linters
make ci       # everything CI does
make help     # all other targets
```

## TDD

New code is written test first:

1. A test that describes the behavior — and fails (red).
2. The minimal implementation that makes it pass (green).
3. Refactoring with green tests.

Code without a test that required it is not accepted. A bug found by hand is first reproduced by a test, then fixed.

| Component | Tests | Where |
|---|---|---|
| `client-php` | PHPUnit 9.6, PHP 7.4–8.4 matrix in CI | `client-php/tests/` |
| `server` | `go test -race` | next to the code, `*_test.go` |
| `ui` | Vitest + Testing Library | next to the code, `*.test.ts` |

Log parsing tests use the fixtures in `spec/fixtures/`, not made-up strings.

## Contracts and generated code

The contract in `spec/` (log formats, OpenAPI) changes first, then the code. Go types (`server/internal/api`, `server/internal/libclient`) and TypeScript types (`ui/src/lib/api/schema.gen.d.ts`) are generated from OpenAPI with `make generate`; `*.gen.*` files are never edited by hand. CI checks that generated code matches what is committed (`make generate-check`).

## Naming

Hungarian notation is banned in all new code: no type prefixes such as `a`, `s`, `i`, `b`, `o`, `m`, `f`, `fn` (`$aVars`, `$sMessage`, `fnDebugLog`, `oSelectedProject`). The type system expresses the type.

| Where | Style | Example |
|---|---|---|
| PHP | PSR-12: classes `PascalCase`, methods and properties `camelCase`, constants `UPPER_SNAKE` | `$projectRoot`, `log()`, `Level::ERROR` |
| Go | idiomatic Go (`MixedCaps`) | `Project.LogDir()` |
| TypeScript / Svelte | `camelCase`, components `PascalCase` | `selectedRequest`, `RequestList.svelte` |
| Log format, config, API, database | `snake_case` | `request_id`, `is_cli`, `messages.level` |

OpenAPI DTO fields stay `snake_case` in TypeScript (no conversion layer).

This is checked automatically: `client-php/tools/bin/check-naming` for PHP and the ESLint `naming-convention` rule for TS/Svelte. A line with a deprecated wrapper that keeps an old name can be excluded with `// naming-check:ignore`.

## UI text

Every user-visible string goes through `i18n.t('key')`. Add the key to both dictionaries in `ui/src/lib/i18n/messages.ts`; the tests check that `en` and `ru` have the same keys, the same placeholders and all plural forms (`Intl.PluralRules`). Native browser dialogs (`alert`, `confirm`, `prompt`), `<select>` and `title` tooltips are not used: there are custom components for them.

Glossary: **request** — one HTTP or CLI run and its log file; **message** — an event inside a request; **context** — the variables of a message; **server** — a machine that hosts projects; **project** — an application on a server.

## Commits

[Conventional Commits](https://www.conventionalcommits.org/) in English:

```
feat(server): import v2 logs from the saved offset
fix(client-php): mask secrets in nested arrays
chore: update golangci-lint to v2.15
```

Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`, `build`. The scope is the top-level directory (`client-php`, `server`, `ui`, `desktop`, `spec`, `docs`).
