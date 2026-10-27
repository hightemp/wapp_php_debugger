# Improvement plan for wapp_php_debugger

**English** · [Русский](IMPROVEMENT_PLAN.ru.md)

The document is based on a review of the code as it was (branch `main`, commit `ecf8962`).
Sections 1–7 describe **what** to do, by topic. **The order of work is set only in section 8** (phases and milestones); section numbers do not follow that order.

**Decisions made:**
- ✅ The viewer (the viewing server) is rewritten in **Go**.
- ✅ The debugger (`Debugger.class.php`) stays in **PHP**.
- ✅ Projects are reached over **SSH**, with support for **several servers and projects** (sections 7.4, 7.5).
- ✅ All code (PHP, Go, Svelte/TS, the log format, the database, configs) **drops Hungarian notation** (section 4.4).
- ✅ The UI is written **from scratch in Svelte 5 + Vite** (an SPA without SSR, embedded into the Go binary). The old EasyUI interface is **removed entirely**, with no transition period and no code carried over.
- ✅ The new UI has a **theme system** (section 6.4).
- ✅ A **desktop version** on Wails is needed: the Go core + the same Svelte UI (section 7.8).
- ✅ The PHP library is compatible with **PHP 7.4+** (old projects are being debugged), section 4.1.
- ✅ Names in the log format, the config, the API and the database are **snake_case** (section 4.4).
- ✅ "Several servers" means **many servers, each with its own projects** (a project lives on one server), section 7.5.
- ✅ Controlling the debugger and reading logs go **over SSH**; an **optional HTTP mode** serves legacy environments without SSH (off by default, only with a token), sections 4.1, 7.4a.
- ✅ SQLite in Go is **`modernc.org/sqlite`** (section 7.3). The desktop is **Wails v3**, falling back to v2 on blockers (section 7.8).
- ✅ **One person** works on the project: phases go one after another, as vertical slices (section 8).
- ✅ The project is **open source**; **the base language is English**, the second one Russian. Code, comments, commits, the CLI, API messages and documentation are in English, user documentation is also translated into Russian; the UI has i18n in both languages, English by default (section 10.11).
- ✅ A **Makefile** as the single entry point for building, testing and releasing (section 3.1) and **GitHub Actions** for CI and releases (section 3.2).

Legend: `- [ ]` — a task; 🔥 — a hotfix of the current version, done right away (phase 1); ↪ — not done in the current code, this is a requirement for the new implementation; ❓ — the decision needs clarification, all such questions are collected in **section 10**.

Terms: **PHP library** — `Debugger.class.php`, included into the project being debugged; **viewing server** — the backend of the web application; **UI** — the web interface in the browser.

---

## 0. Current state (in brief)

The project has two parts:

1. **The client library** — `Debugger.class.php`. It is included into the project being debugged and writes one JSONL file per request: the first line is a fixed-length header (1000 bytes, overwritten when the request ends), then message lines (`sType`, `sMessage`, `oVars`, `oBackTrace`, `aBackTrace`, `iTimestamp`). It also accepts control commands through the GET parameter `debug_msg*` (`status`, `start`, `stop`, `set_config`, `clear`, `info`, `get_file`).
2. **The viewing web application**: `index.php` + `ajax.php` + `controllers/*` + jQuery EasyUI + RedBeanPHP/SQLite. It scans the log folder of a project, imports the files into SQLite (`tdebugfiles`, `tdebugmessages`) and shows three panels: projects → files (requests) → messages → details (the backtrace and the tree of variables).

The idea is good and works, but many places in the code break out of the box, there are serious security holes, and dead code from another project is left over (links/categories/groups/notes/tables).

---

## 1. Critical bugs (the application does not work or crashes)

### 1.1 PHP library `Debugger.class.php`

> The library is already included in working projects, so the items marked 🔥 are fixed right in the current file (cheap fixes that remove fatal errors). The rest ↪ is solved by rewriting the library and by format v2 (sections 4.1, 4.2): fixing it in the old code would be double work.

- [x] 🔥 **`DA_FLAG` and `DA_ROOT_DIR` are not defined** (lines 11–16 are commented out). In PHP 8 using an undefined constant is a fatal `Error`, so `__d()`, `__log()`, `__start()` and `__stop()` crash right after the file is included. They must get defaults (`defined() || define()`) or be replaced with config properties.
- ↪ **`fnGetDirSize()`** (line 133): `"\1"` in double quotes is the character `chr(1)`, not a back reference. The function always returns garbage, so `iLogsSizeLimit` does not work. It needs `'$1'` or, better, computing the size in PHP without `du`.
- [x] 🔥 **`save_debug_message`** (line 225): `json_decode($_POST['sData'])` returns an object, which is then used as an array (a fatal error). Besides, the arguments are passed in the wrong order: `sLang` ends up in the `$sURL` parameter.
- [x] 🔥 **`fastcgi_finish_request()`** (line 376) is called without `function_exists()`. Under Apache mod_php or `php -S` this is a fatal error in a shutdown function.
- [x] 🔥 (minimal: cut the header to 1000 bytes, without `ValueError`) ↪ (complete: format v2 without an overwritten header) **The fixed-length 1000-byte header** (`fnPrintJSONLineWithLength`, line 506). With a long URL or `argv` the header grows beyond 1000 bytes, `fnUpdateHeader()` overwrites the start of the first messages, and the file is corrupted. On top of that, `str_repeat` with a negative number throws `ValueError`.
- [x] 🔥 **`iArraySizeLimit`**: `count($aVar, COUNT_RECURSIVE)` on a scalar throws `TypeError` in PHP 8.
- ↪ **`fnGetSysUser()`** (line 159) is hard-wired to `$_SESSION['USER_INFO']['login']` of one particular project. A user resolver callback in the config is needed.
- [x] 🔥 Call `posix_getpwuid` and `session_id()` only when the extension is there or a session is active.
- ↪ `sleep(1)` in a loop when file names clash (line ~640) slows the request down by a second. The file name must be unique: `msTimestamp-pid-random`.
- ↪ The error handler logs only `E_USER_ERROR`: warnings, notices and deprecations are lost, the previous handler is not called. There is no `set_exception_handler` at all, so uncaught exceptions do not reach the log.
- ↪ Writing without `flock()`: with `fastcgi_finish_request` and parallel sub-requests lines can get mixed up.

### 1.2 Viewing server

> ↪ **Not fixed in the current PHP server**: it is removed entirely after the MVP of the Go version (section 8, phase 5). The list serves as a set of requirements and test cases for the Go implementation (section 7.3): every item is a scenario the new server must handle correctly.

- ↪ **`lib.php` contains code of another project**: `T_CATEGORIES`, `T_LINKS` and `fnBuildRecursive*Links*` are not defined, so `delete_project` → `fnBuildRecursiveProjectsTreeDelete()` crashes. The same in `update_/create_debug_file` and `update_/create_debug_message`: there it is `T_GROUPS`, `tgroups`, `tdebug_files`.
- ↪ **`scan_debug_files`** (`controllers/debug_files.php:12`): the "last file" is looked up across all projects, not the current one, so with several projects files are skipped. The `file_name >` comparison is a string one.
- ↪ The import reads the header with `fread($h, 1000)` (line 43) and does not check that the JSON is valid. The whole file is loaded into memory (`file_get_contents` + `explode`), and every message is saved with two `R::store` calls. On files of 50+ MB this runs out of memory and is very slow.
- ↪ `count_errors` is always `0`.
- ↪ `controllers/tags.php` is not included in `ajax.php`; inside it the SQL is invalid (`DELETE ? WHERE ...`, variable variables `${$aRequest[...]}`).
- ↪ `database.php:24`: the MySQL settings are hard-coded and ignore `config.json`.
- ↪ `index.php` includes scripts that do not exist or are not needed (`speadsheet.js`, `all.js`, `easymde`), and `highlight.js` comes from a CDN without a version (`latest`).
- ↪ `sse.php` does not really stream: it sends one event and closes, EventSource reconnects about every 3 s, and every time a `glob()` over the folder runs. In fact it is hidden polling.
- ↪ `config.php:3` — `error_reporting(E_ERROR | E_PARSE)` hides all warnings, which is why the bugs above are not visible.

---

## 2. Security

The debugger runs on the developer's machine, but its client part is embedded into a **project**, which can end up on stage or in prod. So this matters.

### 2.1 PHP library

- [x] 🔥 **Control through GET without authorization.** Any visitor of the site can call `?debug_msg=start`, `stop`, `clear` or `set_config`. Hotfix: the control API is **off** by default; it is turned on by the constant `DA_HTTP_TOKEN`, and then every request must pass that token (compared with `hash_equals`). A full optional HTTP mode is built in the new library (section 4.1); the main way of control is a config file over SSH (section 7.4).
- [x] 🔥 **Reading arbitrary files**: `?debug_msg=get_file&file=../../../etc/passwd` (line 320). It needs `realpath()` and a check that the path lies inside `sProjectRootPath`.
- [x] 🔥 **Command injection**: `set_config` writes paths from `$_POST`, which are later put into `exec("rm -rf ...")`, `du -s`, `df -h` and `ls ... | xargs rm` (lines 206, 284). `exec` must go entirely, replaced with `glob`/`unlink`/`RecursiveDirectoryIterator`.
- [x] 🔥 **Leaking secrets**: on include, `$_SERVER`, `$_SESSION`, `$_COOKIE` and `$_POST` are logged automatically, that is passwords, tokens, keys from env. Masking by a list of keys (`password`, `token`, `secret`, `authorization`, `cookie`, `api_key`…) is needed, configurable, and the automatic dump of superglobals must be turned on by an option.
- ↪ Replace the `0777` and `0666` permissions of logs and the config with configurable ones (`0775`/`0664` by default). ✅ *Done in library 2.x: `file_mode`/`dir_mode` (phase 6).*
- [x] ↪ Protection against being turned on in prod: an `allowed_envs` option or a check of `APP_ENV`. — *library 2.x (after phase 6, in the phase 7 branch): `allowed_envs` + `env_var` (`APP_ENV` by default); outside the list or without the variable both logs and the HTTP mode are off.*

### 2.2 Viewing server

> The current server works until the MVP of the Go version, so in it we close only what can be exploited from outside without effort (🔥: XSS, CORS, listening on all interfaces). The rest ↪ are requirements for the Go server.

- ↪ **SQL injections**: `fnGenerateFilterRules()` (`lib.php:53`) puts `field` and `value` from the request right into SQL; `fnPagination()` (`lib.php:63`) does the same with `page` and `rows`. Needed: a whitelist of fields, `?` bindings, `(int)` for pagination.
- ↪ **Command injection**: `exec("rm -rf {$sDLP}/{$oDebugFile->file_name}")` (`controllers/debug_files.php:136, 186`).
- [x] 🔥 **XSS**: `s_message` is output as raw HTML (`static/app/modules/debug_messages.js:238`, `debug_message_info.js:59`). Messages contain user input (`$_GET`, `$_POST`), so opening a log can run someone else's JS. Escaping on the front end is needed, and links to files must be built from the already escaped text.
- [x] 🔥 **CSRF + CORS** (hotfix: remove `*`; fully ↪ in Go): nginx sends `Access-Control-Allow-Origin: *` (`docker/nginx/conf.d/vhost.conf:48`), and destructive methods (`clear_files_debug_files`, `delete_*`) are available through GET. Any open tab can wipe the logs. Needed: remove `*`, allow mutations only through POST, add a CSRF token or an `Origin` check.
- [x] 🔥 Listen on `127.0.0.1`, not `0.0.0.0` (`bin_start_server.sh`, the nginx port in `docker-compose.yaml`). ↪ Authorization — in Go (section 7.3).

---

## 3. Cleaning up the repository and infrastructure

> The code of the old server and EasyUI is removed entirely (phase 5), so it is not cleaned up or optimized piece by piece. Dead code in `lib.php`, RedBean (`freeze`, migrations), indexes and `PRAGMA` for the old schema, simplifying `docker/php-workspace` — ↪ not done. The requirements for the schema and the database move to Go (sections 7.3, 7.5).

- [x] **Monorepo layout** — first of all, the paths in the Makefile and CI depend on it (section 10.14):
  ```
  client-php/   — PHP library (the debugger), a composer package
  server/       — Go: cmd/wapp-debugger, internal/*, web/ (embed)
  desktop/      — Wails application on top of server/internal
  ui/           — Svelte + Vite
  spec/         — logformat (JSON Schema v1/v2), openapi.yaml, fixtures
  legacy/       — the current PHP server and EasyUI until removal (phase 5)
  ```
- [x] Personal data: remove `.env` from the repository (it has an email), add `.env.example`; remove the personal mount `~/__SpecFolders__/WorkProjects/front_fast_01` from `docker-compose.yaml`. The email stays in the git history; ❓ *whether to rewrite the history — section 10.35.*
- [x] **Log fixtures v1**: collect real log files of the current format (a regular request, AJAX, CLI, with errors, with a long header, a truncated file), anonymize them and put them into `spec/fixtures/v1/`. The tests of the Go parser and the UI e2e tests are written on them.
- [x] README: fix the name (`wapp_avto_express_debugger` → `wapp_php_debugger`) and typos, briefly describe the new architecture and the status of the rewrite. Full documentation is written for the new components.
- [x] Quality tools for the new code: `composer.json` + PHPStan + PHP-CS-Fixer (`client-php/`), golangci-lint (`server/`), ESLint/Stylelint/Prettier (`ui/`); run through `make` (section 3.1) and GitHub Actions (section 3.2). The `legacy/` directory is not checked by linters.
- [ ] Tests (PHPUnit) for the PHP library: writing and reading the format, masking, size limits, concurrent writes. Server tests are written for Go (section 7.3). — *partly: writing by schema v2, masking, limits, errors, processes and the HTTP mode are covered (phase 6); there is no separate test of concurrent writes — every request has its own file.*

> The Makefile and CI skeleton is created in phase 2, while there is little code. Targets and jobs are added together with the components (phases 4–9). The `legacy/` directory is not part of CI.

### 3.1 Makefile

One entry point for the developer and for CI: workflows call `make <target>` instead of duplicating commands. So a local run and CI behave the same.

- [x] `make` / `make help` prints the list of targets with descriptions by default (self-documenting: `## …` comments on targets).
- [x] Variables: `VERSION ?= $(shell git describe --tags --always --dirty)`, `COMMIT`, `BUILD_DATE`, passed to Go through `-ldflags "-X …"`; `GOOS`/`GOARCH` for cross-builds.
- [x] Tool check (`make doctor`): presence and versions of Go, Node, PHP, Composer, Wails CLI, golangci-lint; a clear message about what is missing.

| Group | Targets |
|---|---|
| Common | `help`, `deps` (go mod download, npm ci, composer install), `doctor`, `fmt`, `lint`, `test`, `build`, `clean`, `ci` (everything CI does, in one command) |
| Code generation | `generate` (OpenAPI → TS types for the UI and Go interfaces through `oapi-codegen`, `sqlc generate`), `generate-check` (generated code matches what is committed) |
| UI | `ui-dev` (Vite dev server with a proxy to Go), `ui-build` (→ `server/web/dist` for `//go:embed`), `ui-lint` (ESLint, Stylelint, `svelte-check`), `ui-test` (Vitest), `ui-e2e` (Playwright, including theme snapshots) |
| Go server | `server-dev` (run with reload, for example through `air`), `server-build` (depends on `ui-build`), `server-lint` (`go vet`, `golangci-lint`, `govulncheck`), `server-test` (`go test -race ./...`), `server-fuzz` (a short run of the parser fuzz tests) |
| PHP library | `php-lint` (PHP-CS-Fixer, PHPStan, the naming rule from section 4.4), `php-test` (PHPUnit) |
| Desktop | `desktop-dev` (`wails dev`), `desktop-build` (for the current OS), `desktop-package` (installers: dmg/NSIS/AppImage/deb) |
| Release | `release-snapshot` (`goreleaser release --snapshot --clean`, a local release check), `docker-build`, `changelog` |

- [ ] Works on Linux and macOS. On Windows — through Git Bash or WSL. ❓ *Whether a native build from `cmd.exe`/PowerShell without make is needed — section 10.31.* — *partly: Linux is checked locally, macOS and Windows (Git Bash) — in CI.*
- [x] Targets are idempotent and need no `sudo`; artifacts go to `dist/`, which is in `.gitignore`.

### 3.2 GitHub Actions: CI and releases

**`ci.yml`** — on every push and pull request:

- [x] `concurrency` cancelling stale runs of one branch; `paths` filters, so that a UI-only change does not run the PHP matrix (while the jobs required for merging still report). — *partly: `concurrency` is there; no `paths` filters.* — *v1.0: the `changes` job (`scripts/changed-areas.sh`, with a test) skips the PHP matrix and the desktop when a change does not concern them; a skipped job counts as passed; tags, new branches and manual runs check everything.*
- [x] Job **php**: a matrix of PHP versions 7.4 / 8.0 / 8.1 / 8.2 / 8.3 / 8.4 (section 10.13) → `make php-lint php-test`; Composer cache.
- [x] Job **go**: `make server-lint server-test server-fuzz`; `setup-go` with a cache; tests of the SSH source against `openssh-server` in a service container. — *partly: lint and tests are there; fuzz and SSH tests are not (SSH — phase 7).* — *v1.0: `make server-fuzz` (10 s per fuzz test of the v1/v2 parsers and the import of third-party logs) in the go job; SSH — `make ssh-e2e` against a real OpenSSH in Docker (phase 7).*
- [x] Job **ui**: `make ui-lint ui-test ui-build`; npm cache.
- [x] Job **e2e**: build the binary (`make build`), run it on log fixtures, run Playwright in all built-in themes; on failure upload the report and screenshots as artifacts. — *phase 8: `make e2e`, v2 fixtures from the real library.*
- [x] Job **generate**: `make generate-check`. Stale OpenAPI or sqlc code breaks the build.
- [x] Job **desktop**: a matrix `ubuntu-latest` / `macos-latest` / `windows-latest` → `make desktop-build` without signing (a check that the desktop builds on every OS).
- [ ] A CI status badge in the README; a protected `main` branch with required checks. — *partly: the badge is there; `main` is not protected.*

**`release.yml`** — on a `v*` tag (SemVer):

- [x] **Server**: GoReleaser builds `wapp-debugger` for linux/darwin/windows × amd64/arm64 (UI embedded), archives, `checksums.txt`, SBOM (syft), changelog. — *partly: archives and `checksums.txt` are there; no SBOM.* — *v1.0: an SBOM of every archive (syft), release notes from Conventional Commits.*
- [x] **Docker**: a multi-arch image (`linux/amd64`, `linux/arm64`) in GHCR, tags `vX.Y.Z`, `vX.Y`, `latest`.
- [x] **Desktop**: a matrix by OS → installers for macOS (`.dmg`, universal), Windows (NSIS `.exe`), Linux (AppImage, `.deb`). Uploaded to the same GitHub Release. ❓ *Signing and notarization — section 10.28.* — *partly: unsigned archives for three OSes for now; installers — phase 9.* — *done in phase 9: `make desktop-package` in `release.yml`, unsigned (section 10.28).*
- [ ] **PHP library**: split the `client-php/` subdirectory into a separate repository and tag there → update on Packagist (section 10.14). — *partly: `release-php.yml` is ready and checked by a trial split (phase 6); the repository, a deploy key and registration on Packagist are needed.*
- [x] Build provenance attestations (`actions/attest-build-provenance`) for the binaries and the image. — *v1.0: archives, the library, `install.sh`, desktop packages and the image (in the registry); the image also has an SBOM and provenance `mode=max`.*
- [x] Release notes: generated from Conventional Commits. ❓ *A manual tag + GoReleaser changelog, or release-please with a release PR — section 10.30.* — *v1.0: per section 10.30 — GoReleaser changelog: groups "Breaking changes", "New", "Fixes", "Security", "Documentation"; test/ci/build/chore are left out.*
- [x] Draft release → check → publish by hand (or publish right away once the process settles).

**Related:**

- [x] `dependabot.yml`: `gomod`, `npm`, `composer`, `github-actions`, weekly, with small updates grouped.
- [x] CodeQL for Go and JS/TS; `govulncheck` and `npm audit --omit=dev` in CI. — *v1.0: `codeql.yml` (on push and weekly); `govulncheck` in `make server-lint` — it found 29 vulnerabilities of the Go 1.26.0 stdlib, the toolchain was raised to 1.26.10; `npm audit --omit=dev` in `make ui-lint`. The first CodeQL run: the CSP test was strengthened, walking directories in project discovery is intended behavior (the alert is closed with an explanation).*
- [x] Actions pinned by SHA, minimal `permissions` for every workflow (`contents: read` by default; `contents: write`, `packages: write`, `id-token: write` — only in release).
- [ ] Secrets (signing certificates, Apple ID, Packagist token) — only in the `release` environment with manual approval. — *partly: the deploy key of the PHP library is in the `release` environment; desktop signing — phase 9.*
- [x] Issue and PR templates, `CONTRIBUTING.md` with `make` commands. — *partly: `CONTRIBUTING.md` is there; no issue/PR templates.* — *v1.0: bug/idea forms, a PR template with a checklist, `SECURITY.md`.*
- [x] **Install one-liners in the README** (the owner's request): `curl` to download the one-file library and to install the server (the script picks the archive for the OS and architecture from the latest release and checks `checksums.txt`) (phase 7, step 1).

---

## 4. Architecture

### 4.1 PHP library as a separate Composer package

Right now the client has to be copied into the project and its paths edited right in the file. Proposed:

✅ **Principle (the owner's decision): `client-php` is a logger script without dependencies.** The `require` of `composer.json` has only PHP and `ext-json` (part of PHP: always built in since 8.0, in 7.4 builds sometimes a module); no packages and no other extensions — the library goes into any 7.4+ project, legacy ones without Composer included, as one file (`wapp-debugger.php`). PSR-3 and Monolog are optional bridges (`suggest`) for projects that already have those packages; they are not part of the one-file build. New features (SQL/Guzzle helpers, `HttpWriter`) are dependency-free too. Checked by tests: `PackageTest` (only `php` and `ext-json` in `require`) and running the built file with `php -n` (without php.ini; of the modules only json is loaded, when it is not built in).

- [x] Move the library into `client-php/` of the monorepo (section 3), namespace `WappDebugger\`, PSR-4. ✅ **Minimum PHP version 7.4** (section 10.13). Consequences:
  - not allowed: `enum`, `match`, union types, `mixed`, nullsafe `?->`, named arguments, constructor promotion, `readonly`, attributes, `str_contains`/`str_starts_with` (without a polyfill);
  - allowed: typed properties, arrow functions, `??=`, spread in arrays;
  - log levels are a class with constants (`Level::ERROR`), not an enum;
  - PHPStan with `phpVersion: 70400`, PHPUnit 9.6 (works on 7.4–8.x), PHP-CS-Fixer with rules for 7.4;
  - CI runs the tests on 7.4 and on every 8.x version, so that there are no deprecation warnings on newer PHP.
- [x] Split responsibilities: — *phase 6; `Writer` without an interface — a disabled `Debugger` instead of `NullWriter`, peak memory and duration — in `request_end` without a separate `Profiler`; `HttpWriter` — phase 10.*
  - `Config` — loading from an array, env or JSON, defaults;
  - `Writer` (an interface) → `JsonlFileWriter`, `NullWriter`; `HttpWriter` (push to the server) — later, when Go gets an HTTP receiver (phase 10);
  - `Serializer` — safe serialization of variables: limits of depth, string length and number of items, objects (class + properties, private ones included), recursion, resources, closures, exceptions;
  - `Masker` — masking secrets;
  - `ErrorHandler` — errors, exceptions, fatal errors in shutdown;
  - `Timer` and `Profiler` — timers, memory (`memory_get_peak_usage`), request duration;
  - `HttpEndpoint` — ✅ **an optional HTTP mode for legacy environments without SSH** (section 10.22). Off by default, and without a token it does not turn on at all. A required token (≥ 32 characters, `hash_equals`, only in the `X-Debugger-Token` header), an optional IP allowlist, rate limiting. Commands: `status`, `enable`/`disable`, `list` (the list of log files), `read` (reading a file from an offset), `clear` (only inside the log folder). No `get_file` and no `set_config` with paths: sources and the config are reachable only over SSH. Responses in JSON, the format is described in OpenAPI (`spec/`). The main way of control is writing the config over SSH (section 7.4).
- [x] Do no work on include: no `du`, `df` or reading the config on every request when the debugger is off. Checking the flag must cost exactly one `is_file()` or reading a constant.
- [x] Compatibility: keep the global `__d()`, `__log()`, `__start()`, `__stop()` as thin wrappers (backward compatibility), add `Debugger::log()`/`info()`/`warning()`/`error()`.
- [x] **A PSR-3 adapter** (`Psr\Log\LoggerInterface`) and **a Monolog handler**, to plug into Laravel, Symfony and any project with Monolog without code changes. Because of PHP 7.4 two dependency lines must be supported: `psr/log` 1.x (PHP 7.4) and 2.x/3.x (PHP 8, other signatures), Monolog 2 (an array) and Monolog 3 (`LogRecord`, PHP 8.1+). The right adapter is chosen at load time; all combinations are covered by the CI matrix.
- [ ] Helpers for common cases: logging SQL (a wrapper over PDO / an ORM event) with duration, logging HTTP requests (Guzzle middleware). — *moved to phase 10 (integrations, section 5.3).*
- [x] **A one-file build** (the owner's request): `client-php` is packed into one `wapp-debugger.php` (all classes and global functions, no autoloader) to be included with a single `require` in projects without Composer; the file is built by `make`, checked by the same tests and attached to every release (phase 7, step 1).

### 4.2 Log format v2

Problems of the current format: a fixed-length header overwritten in place; the full `aBackTrace` in every message (it bloats files a lot); no schema version.

- [x] **The specification first, then the code.** Format v1 (as it is) and v2 are described by JSON Schema in `spec/logformat/` with examples. This is the shared contract of the PHP writer and the Go parser; both validate the fixtures against the schema in CI.

- [x] Every line is a self-contained event with a `type` field: `request_start`, `message`, `timer`, `error`, `request_end`. The header is not overwritten; the outcome of the request (response code, time, memory) is written as a separate `request_end` line.
- [x] A `v: 2` field for the format version; the server reads both formats.
- [x] Field names without Hungarian notation, in ✅ `snake_case` (`sType` → `level`, `oVars` → `context`, `iTimestamp` → `ts`, `bIsCli` → `is_cli` and so on, see section 4.4).
- [x] `request_id` (UUID/ULID), `parent_request_id` to link AJAX ↔ page, CLI ↔ cron. — *the parent is passed in the `X-Wapp-Debugger-Parent` header and the `WAPP_DEBUGGER_PARENT` variable (phase 6).*
- [x] Backtrace: short by default (N frames, paths relative to the project root), full — by an option or only for errors. — *`trace_frames`, 10 by default; exceptions get the full one.*
- [x] Tags as a separate `tags: []` field instead of `#timediff #sql_query` in the message text (the old syntax is parsed for compatibility).
- [x] Rotation and retention: limits on the number of files, the total size and age; the oldest are deleted in PHP, without `exec`.

### 4.3 Viewing server

> ✅ Decided: the server is rewritten in Go (section 7). The items below are implemented in Go. The current PHP server gets only the fixes from sections 1.2 and 2.2 while it runs, and is then removed.

- [x] Instead of the chain `if ($sMethod == ...)` + `die(json_encode())`, a router and handlers with one JSON response shape and error codes (now errors silently turn into `null`).
- [x] Input validation, correct HTTP methods.
- [x] **Import**: streaming reads (`bufio.Reader`), batched inserts in one transaction through a prepared statement, incremental import (remember the offset of the last byte read, to read the rest of a growing file).
- [ ] Consider an option **without double storage**: for small volumes read JSONL directly with an index (line offsets), and use SQLite only as an index and FTS. Now every message is stored twice: in the file and in the database.
- [x] Full-text search: SQLite FTS5 over the message text and the context (variables).
- [ ] Automatic background import (a watcher goroutine: `fsnotify` locally, polling over SFTP for SSH) instead of a "Scan" button. — *partly: locally fsnotify with fallback polling (phase 5); SFTP — phase 7.*
- [x] Real SSE: a long-lived connection, `Last-Event-ID`, heartbeat. New requests and messages appear in the UI in real time (*live tail* mode).

### 4.4 One naming style: dropping Hungarian notation

Right now almost every name carries a type prefix: `$aVars`, `$sMessage`, `$iCount`, `$bIsOnFlag`, `$oProject`, `$mValue`, the methods `fnDebugLog()`, `fnGetInstance()`, the JS getters `oComponent`, `fnReload`, `sURL`, the log fields `sType`/`oBackTrace`/`aBackTrace`/`iTimestamp`, the database columns `s_type`/`o_vars`/`i_timestamp`/`o_back_trace`. Types are already expressed by the type system (PHP type hints, TypeScript, Go), and the prefixes lie after refactoring (for example, `aVars` is actually cast to `object`). They go **everywhere**.

**Rules:**

| Layer | Style | Example, before → after |
|---|---|---|
| PHP library (PSR-1/PSR-12) | classes `PascalCase`, methods and properties `camelCase`, constants `UPPER_SNAKE` in classes | `$sProjectRootPath` → `$projectRoot`; `fnDebugLog()` → `log()`; `fnGetInstance()` → `instance()`; `$bIsOnFlag` → `$enabled`; `$iArraySizeLimit` → `$maxArrayItems`; `DWS_ERROR` → `Level::ERROR` (a class constant: enums are not available in PHP 7.4) |
| Log format v2 | ✅ `snake_case`, no prefixes | `sType` → `level`, `sMessage` → `message`, `oVars` → `context`, `oBackTrace` → `source`, `aBackTrace` → `trace`, `iTimestamp` → `ts`, `bIsCli` → `is_cli`, `iRequestTimestamp` → `request_id`, `sLang` → `lang` |
| Config `dws_debugger.json` | the same rules as the format | `sMessagesFilesPath` → `logs_dir`, `iLogsSizeLimit` → `max_logs_size` |
| Go server | idiomatic Go (`MixedCaps`, no `Get` prefixes), `golangci-lint` with `revive`/`stylecheck` | `fnGetProjectDebugLogPath` → `Project.LogDir()` |
| Svelte / TypeScript | `camelCase`, components `PascalCase`, no `o`/`s`/`fn` | `oSelectedProject` → `selectedProject`; `fnReload()` → `reload()` |
| Database (the new schema in Go) | `snake_case`, no type prefixes and no `t` on tables | `tdebugmessages.s_type` → `messages.level`; `tdebugfiles` → `requests`; `tprojects` → `projects` |
| API (OpenAPI) | `snake_case`, like the format | `debug_file_id` → `request_id` |
| DTOs in TypeScript | API fields stay `snake_case` (types are generated from OpenAPI, without a conversion layer); local variables and functions — `camelCase` | `request.status_code`, but `const selectedRequest` |

**Tasks:**

- [x] Record the rules in `CONTRIBUTING.md` (the table above + a glossary: "request", not "debug file"; "message"; "context").
- [x] PHP library: renaming while moving into a package (section 4.1). For automation: Rector with the `RenamePropertyRector`/`RenameMethodRector` rules from a generated name map, then a manual read-through. — *by hand while rewriting, without Rector.*
- [x] ✅ Backward compatibility of the PHP API (section 10.23): the global `__d()`, `__log()`, `__start()`, `__stop()` stay **forever** (short helpers, not Hungarian notation). The functions `fnDebugLog()`, `fnDebugAddFoundField()`, the `fn*` methods and the `DWS_*`/`DEBUG_MSG` constants stay as wrappers with `@deprecated` (with `E_USER_DEPRECATED` once per request) and are removed in the next major version (3.0).
- [x] Log format: the v1 parser in Go maps old fields to new ones (`sType` → `level` …), so old logs are read without conversion.
- [x] Config: the loader accepts old keys with a warning in the log.
- [x] A CI check so that the prefixes do not come back (only for new code, `legacy/` is excluded):
  - PHP: a PHP_CodeSniffer/PHPStan rule on names of variables and properties (`/^(a|s|i|b|o|m|f|fn)[A-Z]/` is forbidden);
  - TS/Svelte: ESLint `@typescript-eslint/naming-convention` with the same ban (`snake_case` is allowed for properties of types from `spec/`);
  - Go: `revive` (`var-naming`) + a custom rule in `golangci-lint` if needed.
- [x] The Go server and the Svelte UI are written in the new style from the start. The old PHP server and EasyUI are not renamed: they will be removed.

---

## 5. New features

### 5.1 UI/UX

- [x] **Live tail**: new requests appear on top automatically, there is a "pause" button. — *v0.6: new requests appear by themselves; pause counts what arrived and shows it on a button.*
- [ ] Filters by type (error/warning/info/message), tag, URL, HTTP method, response code, duration, CLI/AJAX/DIRECT; saved filters. — *partly: level, CLI/Web, "only with errors", URL and text through search (phase 5); method, code, duration, AJAX — no.* — *v0.6: a filter line `level: tag: method: status: duration> ajax: sapi: pinned: url:` with autocompletion; no saved filters.*
- [x] Highlighting and error counters in the request list (now `count_errors = 0`).
- [x] **Timeline/waterfall** of a request: messages and timers on a time scale, "where it is slow" is seen at once. The data (`iTimestamp`, `#timediff`) for it is already there. — *v0.5: a "Timeline" tab: timers, SQL and HTTP as segments `[ts − duration, ts]`, other events as dots.*
- [x] An SQL tab: all queries of a request, their duration, duplicates (N+1). — *v0.5: queries in order with their time, the slowest and repeated ones (N+1) marked; recording — `Sql::watch($pdo)` and `Debugger::sql()`.*
- [x] Viewing variables: a handy JSON viewer with collapsing, search and copying of the path and the value (now the treegrid turns everything into strings, null, bool and numbers look the same). — *a tree with types (phase 5); v0.5: search over keys and values, copying the path (`['user']['id']`) and the value.*
- [x] Viewing the source around a line from the backtrace (highlighted), safely and only inside the project. — *v0.5: "Show code" at the place of a message and at every frame; the file is read from the project server (locally or over SFTP) only inside the root, links are resolved; highlighting with highlight.js.*
- [x] Configurable "open in editor" links: VS Code, PhpStorm (`phpstorm://open?file=&line=`), Cursor, Sublime. Now only `<scheme>://file<path>` is supported.
- [x] Copying a request as cURL, replaying a request. — *v0.7: "Copy as curl" from the recorded `$_SERVER` (headers), `$_COOKIE`, `$_POST`; GET is opened again in the browser. The library does not write the request body (JSON) — curl does not have it; hidden values stay `***`, a notice says so.*
- [x] Comparing two requests (a diff of variables, for example `$_SESSION` before and after). — *v0.7: `#/requests/42/compare/41`: the fields of both requests and, for every superglobal, added, removed and changed paths.*
- [x] Export and import (JSONL/ZIP) of a request, to hand it to a colleague. — *v0.7: JSONL in format v2 (`GET /requests/{id}/export`, `POST /projects/{id}/import`), the import also accepts a log file of the library; ZIP is not needed — child requests are exported separately.*
- [x] Themes (section 6.4) and keyboard shortcuts (j/k over messages, `/` for search). — *themes — phases 4–5; shortcuts — v0.6.*
- [ ] Pinning and notes on requests; using the existing tag tables, let requests be marked with tags. — *v0.6: pinning and a note in the request header, a mark in the feed, `pinned:yes`; retention does not delete them; no tags on requests.*
- [x] Notifications about new errors: in-app toast notifications; system ones only with consent (section 6.3), native ones in the desktop (section 7.8). — *v0.6: a toast about a failed request the feed does not show right now; system ones — in the desktop (phase 9).*

### 5.2 Data sources

- [x] A JS client (`debugger.js`) for the front end: `window.onerror`, `unhandledrejection`, `console.*` → to a server endpoint, tied to the `request_id` of the page. The `sLang` field (in v2 — `lang`) and the `save_debug_message` stub are already there for it. — *v0.8: the receiver serves `debugger.js` (`/ingest/debugger.js`), `Debugger::jsTag()` prints the tag; errors with a stack, unhandled rejections and `console.error`/`warn` — a request with `lang: js`, a child of the PHP request of the page; the token in the query, a text/plain body — no CORS preflight, `sendBeacon` works. The contract is `spec/fixtures/v2/js-page.jsonl`, e2e in Chrome.*
- [x] Receiving logs over HTTP right on the viewing server (without a shared folder, handy for remote and Docker environments). — *v0.8: `serve --ingest-addr` — a separate address with only the receiver (`spec/ingest.yaml`), a token per project (shown once, SHA-256 in the database); `HttpWriter` of the library (`push_url`, `push_token`, no extensions); sending again does not duplicate events.*
- [x] Importing third-party logs: `php_errors.log`, Laravel `storage/logs/*.log`, the nginx error log. The `T_LOG_FILES`/`T_LOG_MESSAGES` tables are already declared but not used. — *v0.8: "Import request" accepts `php_errors.log`, a Laravel log and the nginx error log (the format is told by the first line): the file is one request with its name, the entries are messages; a one-time import, the tail is not followed in real time.*

### 5.3 Integrations

- [x] Ready instructions and providers for Laravel (ServiceProvider + Monolog channel), Symfony (bundle), WordPress, Bitrix, plain PHP. — *v1.0, by the owner's decision — instructions and thin files without dependencies: the `Bridge\LaravelChannel` channel, a WordPress mu-plugin, a file for Bitrix's `init.php`, a `config/packages` config for Symfony, plain PHP — `client-php/integrations/README.md`; WordPress and Bitrix are checked in processes with a fake CMS.*
- [x] Xdebug links: a "run the request again with `XDEBUG_SESSION`" button. — *v0.7: a "With Xdebug" switch adds `XDEBUG_SESSION=1` to "Open again" and to curl.*
- [x] Library helpers: logging SQL (a wrapper over PDO / an ORM event) with duration, logging HTTP requests (Guzzle middleware) — moved from section 4.1; with them the SQL tab in the UI (section 5.1). — *v0.5: `Sql::watch($pdo)`, `Debugger::sql()`/`http()`, `Bridge\GuzzleMiddleware` — without dependencies.*

---

## 6. New web interface in Svelte

> Here the "client" is the web interface for viewing logs (now `index.php` + `includes/layout.php` + `static/app/modules/*.js` on jQuery EasyUI). The PHP library `Debugger.class.php` has nothing to do with this stage.
>
> ✅ **Decided:** the UI is written **from scratch** in Svelte 5 + Vite. Nothing is carried over from the old interface: no code, markup, styles or icons. It serves only as a list of features that must stay (section 6.6). EasyUI, jQuery and all of `static/app` are deleted.

### 6.1 Why rewrite

- EasyUI is outdated, pulls in jQuery and limits the UI: the variables treegrid breaks on styles (see the comment in `includes/layout.php`), the layout is hard-coded in pixels (`width:540px`, `440px`).
- The modules `debug_files.js`, `projects.js`, `debug_messages.js`, `tags.js` are nearly the same: 300 lines each of `$('#id')` getters and dialog wiring. Every new feature means one more copy of that code.
- No virtualization: a table of 10k+ messages is slow, so pagination is a must.
- Markup, state and requests are mixed up, there are no tests.

### 6.2 Stack

- [x] ✅ **Svelte 5** (runes) + **TypeScript** + **Vite**, an SPA without SSR and without SvelteKit. Routing — a light client router (hash or the History API with a fallback to `index.html` in the Go server).
- [x] Built into static files embedded into the Go binary (`embed.FS`, see section 7).
- [x] A typed API client (`fetch` + types, generated from the OpenAPI contract where possible, see section 6.6).
- [x] State: Svelte runes and stores. The selection state (project → request → message, filters) lives in the URL, so that any request or message can be linked to.
- [x] Realtime: a wrapper over `EventSource` (reconnection, `Last-Event-ID`) as a store. ❓ *SSE or WebSocket — section 10.7.*
- [x] Virtualized lists (for example, `@tanstack/svelte-virtual`) for requests, messages and the tree of variables. — *phase 8: our own `VirtualList` without dependencies (rows of one height, only the visible ones in the DOM) for the feed and messages; all messages of a request are loaded, page by page (they used to be cut at 500); the tree of variables shows big nodes 200 children at a time. Checked by e2e on a request with 3000 messages.*
- [x] Code highlighting: Shiki or highlight.js, bundled locally, not from a CDN "latest". — *→ phase 10, together with source previews (section 5.1): nothing to highlight yet.* *Done in v0.5: highlight.js 11.12 (the core and PHP, JS, JSON, SQL, HTML) in the UI build.*
- [x] Tests: Vitest (components, stores) + Playwright (e2e: open a project → a request → a message → variables). — *Vitest — since phase 4; Playwright e2e — phase 8 (`make e2e`, the **e2e** job in CI): the user's path, a link to a message, three built-in themes.*
- [x] Linting: ESLint + Prettier + `svelte-check` in CI.

### 6.3 Replacing all default browser UI with custom components

Every element is built as our own component in one style. Colors, spacing and fonts come only from theme tokens (section 6.4). Accessibility is kept: focus, keyboard, ARIA.

| Native / default element | Where now | Replacement |
|---|---|---|
| `alert` / `confirm` / `prompt` (now through `$.messager`) | `lib.js:15`, `debug_files.js:140`, `projects.js:124`, `debug_messages.js:126`, `tags.js:154` | `Dialog` / `ConfirmDialog` with a promise API (`await confirm(...)`), a focus trap, Esc/Enter |
| The system dialog "Open the vscode:// application?" | `editor_url` links | Our own "Open in…" popover (VS Code / PhpStorm / Cursor / copy the path) that remembers the choice |
| `<select>` / combobox | the project form (`link_type`) | `Select` / `Combobox` with search |
| The `title` attribute (native tooltips) | toolbar buttons without labels | `Tooltip` with a delay and the shortcut in the text |
| The browser context menu | lists of files and messages | `ContextMenu` (copy, open in the IDE, delete, compare, pin) |
| Native scrollbars | all panels | Thin styled scrollbars through CSS (`scrollbar-width`, `::-webkit-scrollbar`), without JS emulation. ❓ *section 10.5* |
| `<input type="checkbox/radio">` | filters | `Checkbox`, `Switch`, `SegmentedControl` |
| `<input type="date">` | a time filter (new) | `DateRangePicker` with presets ("last 15 min", "today") |
| `<input type="file">` | JSONL/ZIP import (new) | A drag & drop zone + a button |
| Web Notifications API | the stub in `notification_manager.js` | In-app `Toast`; system notifications only with explicit consent in the settings |
| Native dragging of dividers | `split:true` in EasyUI | `SplitPane` that remembers sizes |
| Native in-page search (Ctrl+F) does not work in virtualized lists | — | Our own search `/` + a command palette `Ctrl+K` |
| Native `<input type="color">` and font picking | — | Our own theme editor (section 6.4) |

- [x] Collect the components in `ui/src/lib/components` and make a showcase for development (a `/dev/components` page or Storybook). ❓ *Write everything ourselves or take a headless library (Bits UI / Melt UI) and style it — section 10.4.* — *v0.9: the components are our own (in `ui/src/components`); the showcase is `#/dev/components` (and "Component showcase" in `Ctrl+K`): every component in all built-in themes side by side, live. `ColorField` was added instead of `<input type="color">`.*

### 6.4 Themes

**Architecture:**

- [x] Design tokens as CSS custom properties: colors (`--bg`, `--surface`, `--text`, `--muted`, `--border`, `--accent`, `--danger`, `--warning`, `--info`, `--success`), colors of message levels (`--level-error`, `--level-warning`, `--level-info`, `--level-message`), syntax and JSON highlighting (`--syntax-key`, `--syntax-string`, `--syntax-number`, `--syntax-null` …), radii, shadows, spacing, fonts.
- [x] Not a single hard-coded color in components. Checked in CI by Stylelint (`color-no-hex` outside theme files, `declaration-property-value-allowed-list`). — *v0.9: Stylelint in `make ui-lint` forbids hex, named colors and color functions (`rgb()`, `hsl()` …) everywhere except `src/styles/theme.css`; the shade under a dialog became the `--backdrop` token.*
- [x] A theme is a file of tokens (TS/JSON → CSS variables on `:root[data-theme="…"]`). Switching is instant, without a reload.
- [x] No "flash" of a wrong theme on load: a small inline script in `index.html` sets `data-theme` before the application mounts. — *done differently: not inline but a blocking `theme-init.js` — inline scripts are forbidden by the CSP (phase 5).*
- [x] Code highlighting (Shiki) and the JSON tree take their colors from the same theme. Every theme defines a pair: a UI theme + a highlighting theme. — *the JSON tree and code highlighting — the `--syntax-*` tokens of each theme (v0.5).*

**Set of themes:**

- [x] Built in: **Light**, **Dark**, **System** (follows `prefers-color-scheme`), **High contrast**. ❓ *Whether extra presets are needed (Solarized, Nord, Dracula, Monokai and so on) — section 10.24.*
- [x] The accent color is chosen separately over any theme. — *v0.9: "Appearance" → accent color (a color of your own or "As in the theme"), a warning when the contrast with the theme background is below 3:1.*
- [x] The environment color over the theme: a `stage`/`prod` stripe or frame (section 7.5) is visible in any theme and is not covered by the accent. — *v0.9: a frame around the whole window in the `--env-stage`/`--env-prod` color when a project or the feed of a stage/prod server is selected; it does not catch clicks; turned off in "Appearance".*

**Display settings (together with the theme):**

- [x] Density: compact / regular (row height in the request and message lists). — *v0.9: the `--row-padding-y`, `--row-line-height` tokens; virtual lists re-measure a row when density and font change.*
- [x] The interface font size and a monospace font for code and variables (the system stack by default; with fonts of your own — local files, no external CDNs). — *v0.9: size 12–18 px; the code font is the name of a font installed on the computer (only names, quotes and commas), followed by the system monospace stack; no font files are loaded.*
- [x] Respect `prefers-reduced-motion` and `prefers-contrast`. — *v0.9: the "System" theme with `prefers-contrast: more` is the high-contrast one, it follows the OS without a reload.*

**Custom themes:**

- [x] A theme editor in the settings: editing tokens with a live preview on a mock-up (the request list, a message, JSON, code), a reset to the base theme. — *v0.9: "Appearance" → "Theme editor": colors over a light, dark or high-contrast base by group, a mock-up of the viewer inside the editor itself (a theme works on any element with `data-theme`), WCAG AA contrast of all pairs live, resetting a token and the whole theme; up to 20 themes.*
- [x] Export and import of a theme as a JSON file; schema validation on import. ❓ *Keep custom themes on the server (in the Go server settings, shared by all browsers) or only in the browser — section 10.25.* — *v0.9: the `<id>.theme.json` file is a `CustomTheme` object from `spec/openapi.yaml`; the import checks the name, the base and the colors, and skips tokens of newer versions. Themes and all appearance settings are on the server (`GET/PUT /api/v1/settings`, a `settings_updated` event to other windows), `localStorage` is a cache for the first frame (`theme-init.js`).*

**Quality:**

- [x] An automatic contrast test: for every built-in theme check the pairs "text/background", "level/background", "highlighting/background" against WCAG AA (4.5:1 for text, 3:1 for large elements and icons).
- [x] Playwright visual snapshots of the main screens in all built-in themes. — *v0.9: `make e2e-visual` compares the viewer in all themes, the showcase, "Appearance" and the theme editor with `ui/e2e/__screenshots__` in the Playwright Docker image (the same fonts locally and in CI); `make e2e-visual-update` takes them again.*
- [x] A component showcase (section 6.3) with a theme switch. — *v0.9: all themes side by side or one of your choice.*
- [x] Switching the theme from the command palette (`Ctrl+K` → "Theme: …") and with a shortcut. — *v0.6: from the palette; no separate key.*

### 6.5 Improving the functionality in the new UI

Besides the features from section 5.1, which are implemented in the new UI right away:

- [ ] Layout: servers → projects → requests → messages → details (section 7.5). Panels can be resized and collapsed; there is a compact mode for a narrow screen. — *partly: projects → requests → message → details are there; no servers in the tree and no resizable panels.*
- [ ] Tabs of message details: **Message**, **Variables** (a JSON tree with types, search, copying the path and the value), **Backtrace** (with a source preview), **Raw JSON**.
- [ ] Tabs of request details: **Overview** (URL, method, code, time, memory, user), **Timeline**, **SQL**, **Errors**, **Superglobals**. — *partly (v0.5): "Messages", "Timeline", "SQL", "Superglobals"; the overview is in the request header; no separate errors tab.*
- [x] A command palette (`Ctrl+K`): going to a project or a request, actions, settings. — *v0.6: projects, servers, pinned and failed requests, windows, themes, languages, settings, the feed filter.*
- [x] Keyboard shortcuts for everything (j/k, Enter, `/`, `Esc`, `g p` — projects), help on `?`. — *v0.6: j/k, Shift+J/K, Esc, /, g p, ?, Ctrl/⌘+K.*
- [x] A filter line with syntax (`type:error tag:sql duration>500 url:/api/*`) and autocompletion. Right now a similar `type:error` syntax exists only in the PHP library. — *v0.6.*
- [x] Several open requests in tabs and a mode comparing two requests side by side. — *v0.7: up to eight tabs above a request; the ⇄ button compares a tab with the open request.*
- [ ] A project settings page instead of a modal window: the server, paths, path mapping (server/container ↔ a local copy for the IDE) with a "the path exists" check, the editor choice, retention.
- [ ] A servers page: adding an SSH connection (a host from `~/.ssh/config`, a key, a jump host), a "Test connection" button, the status and latency (section 7.4).
- [ ] A status indicator: the connection to the live stream and to every server (online/offline/reconnecting), the number of files not yet imported (now `#project-files-info`), disk space.
- [ ] Empty states and onboarding: "no projects → how to include `Debugger.class.php`" with a ready snippet. — *partly: an empty feed hints at what to do; no onboarding.*
- [x] i18n (ru/en). ❓ *section 10.11.*

### 6.6 Order of work

1. [x] Fix the **API contract v1** (OpenAPI): REST resources `servers`, `projects`, `requests`, `messages`, `context`, `stream`. The Go backend implements this contract (section 7); field names without Hungarian notation (section 4.4). — *done (phases 3–5).*
2. [x] Create the `ui/` project from scratch (Vite + Svelte 5 + TS), set up linters, tests and the build into `server/web/` for `//go:embed`. — *done (phases 3–5).*
3. [x] Design tokens and themes (section 6.4), then the component set (section 6.3) with a showcase. — *done (phases 3–5).*
4. [x] Screens and features as **vertical slices**: an endpoint in Go + a screen in the UI + an e2e test in one go (one developer, section 10.34). The data comes from the Go server on fixtures; a mock server from OpenAPI is not needed. — *done (phases 3–5).*
5. [x] The required minimum of features that the old interface has and the new one must have (used as a checklist, not as a model for carrying code over): the list of projects, finding new logs, the list of requests, messages, the backtrace, variables, clearing logs, links to the IDE, the counter of files not yet imported. — *done (phases 3–5).*
6. [x] Delete the old UI entirely, without a transitional `/legacy` mode: `index.php`, `includes/`, `static/app/` (EasyUI, jQuery, `node_modules`, `modules/*.js`, the 4000-line `icons.css`, the famfamfam icons, `easymde`, `jspanel3`, `speadsheet.js`), `screenshots/` (to be replaced with new ones). Icons in the new UI are an SVG set (for example, Lucide). — *done (phases 3–5).*

---

## 7. Viewer backend in Go

> ✅ **Decided.** The **viewing server** is rewritten in Go (`index.php`, `ajax.php`, `controllers/*`, `sse.php`, `bin.php`, RedBean/SQLite). **The debugger `Debugger.class.php` stays in PHP**: it runs inside the PHP project being debugged.

### 7.1 What Go gives

- **One binary** `wapp-debugger` with the Svelte UI embedded (`embed.FS`): no PHP-FPM, nginx and Docker with 15 extensions needed. Run: `wapp-debugger serve --projects ./projects`.
- **Background processes**: watching log folders with `fsnotify`, incremental import of growing files and real SSE/WebSocket. In PHP-FPM long-lived connections hold workers; in Go this is cheap.
- **Speed and memory**: streaming JSONL parsing over gigabyte logs, batched inserts, parallel import of several projects.
- **Receiving logs over HTTP/UDP** (section 5.2) comes naturally, without a shared file system.
- Cross-platform builds through goreleaser (Linux/macOS/Windows) and a small Docker image (distroless/scratch).
- A desktop application on Wails from the same Go core and the same Svelte UI (section 7.8).

### 7.2 Downsides and risks

- A second language in the project: the library stays in PHP, the server becomes Go. The entry barrier for contributors who are PHP developers is higher.
- Work on the PHP server and the old UI will be thrown away, so their development is **frozen**. Only critical bugs and security holes are fixed (sections 1.2, 2.2) until the Go server and the new UI are ready. After that the PHP server and EasyUI are removed at the same time.
- The logic of path mapping Docker ↔ host (`path`, `global_path`, `relative_path`, `lib.php:fnGetProject*Path`) and of editor links must be carried over carefully.

### 7.3 Stack and structure

- [x] ✅ Go 1.26 (`go.mod`; at the time of phase 2 versions 1.26 and 1.27 are supported, an old local Go downloads the needed toolchain itself), `net/http` with the new routing (or `chi`), `log/slog`. The linter is golangci-lint v2 (v1 is not supported by the current `golangci-lint-action`), its version is pinned in the Makefile.
- [x] ✅ SQLite: **`modernc.org/sqlite`** (section 10.6): pure Go, the server binary builds without CGO for every OS with one command, FTS5 is on, and the speed is more than enough for a log debugger. The driver is used only through `database/sql` in `internal/store`, so if performance becomes a problem, switching to `mattn/go-sqlite3` is a change of one file. The desktop needs CGO anyway (WebKit/WebView2), but that does not get in the way. Migrations are embedded into the binary (`goose` or our own runner on `embed`).
- [x] Database access through `sqlc` or `database/sql` with hand-written queries, no ORM.
- [ ] Packages:
  - `cmd/wapp-debugger` — the CLI: `serve`, `import`, `watch`, `projects add/list`, `purge`;
  - `internal/logformat` — the parser of formats v1 (the current one, with the 1000-byte header) and v2 (section 4.2);
  - `internal/source` — an abstraction of a log source: `LocalSource` (fsnotify), `SSHSource` (SFTP), `HTTPPullSource` (the legacy HTTP mode of the library, section 7.4a), later `PushSource` (an agent, section 7.6); a common interface `List/Stat/ReadFrom(offset)/Remove/SetEnabled`;
  - `internal/ssh` — a pool of SSH connections, `~/.ssh/config`, agent, known_hosts, ProxyJump, keepalive and reconnection;
  - `internal/ingest` — the import scheduler over all sources, import from an offset, the HTTP receiver;
  - `internal/store` — SQLite, FTS5, retention;
  - `internal/api` — REST by the contract from section 6.6 + SSE;
  - `internal/pathmap` — path mapping and IDE links;
  - `internal/codeview` — safe reading of project sources for previews (locally or over SFTP; only inside the project root, `filepath.Rel` + `EvalSymlinks`/`realpath` on the remote side);
  - `web/` — the built Svelte, `//go:embed`.
- [ ] Config: a YAML/TOML file + flags + env. Servers and projects are set in the config or added through the UI (section 7.5). — *partly: flags and env so far (`WAPP_DEBUGGER_TOKEN`), projects — in the database through the UI/API; no config file.*
- [x] Secure by default: listen on `127.0.0.1`, a token for the API and for receiving logs, CORS off, mutations only through POST/DELETE.
- [x] Tests: unit tests of the parser on real samples (`tmp/message.json`, `tmp/head.json`), integration tests of the API (`httptest`), a fuzz test of the JSONL parser, integration tests of the SSH source against a test SSH server (`gliderlabs/ssh` or an `openssh-server` container in CI).
- [x] CI and releases — through the Makefile and GitHub Actions (sections 3.1, 3.2).

### 7.4 Connecting to projects over SSH

Right now the viewer reads logs only from a local folder or a Docker mount. Hence the mapping `path`/`global_path`/`path_to_debug_log`/`global_debug_log_path` and the personal mount in `docker-compose.yaml`. An SSH connection lets you look at logs on any dev/stage server without shared folders and without opening HTTP ports.

**Connection and authentication:**

- [x] A client on `golang.org/x/crypto/ssh` + `github.com/pkg/sftp`.
- [x] Reading `~/.ssh/config` (`Host`, `HostName`, `User`, `Port`, `IdentityFile`, `ProxyJump`), so that a server can be added just by its alias.
- [ ] Authentication: ssh-agent (`SSH_AUTH_SOCK`), keys from files (with a passphrase asked for in the UI). ❓ *Passwords and storing secrets — section 10.19.* — *partly: ssh-agent and keys, keys from `IdentityFile` first, `IdentitiesOnly` (phase 7); for a key with a passphrase — a hint to add it to ssh-agent, no passphrase input in the UI.*
- [x] **Host key verification** through `~/.ssh/known_hosts` (`knownhosts`). No `InsecureIgnoreHostKey`. A new host is confirmed in the UI with its fingerprint shown (TOFU). — *phase 7: the user's known_hosts + our own file; a new key is confirmed in the UI by its fingerprint with the type and a hint about which file to compare on the server; the key is trusted only if the server still shows it; a changed one is refused; a known key type is requested, ED25519 for new hosts.*
- [x] Jump hosts / bastions (`ProxyJump`, a chain).
- [x] Keepalive, reconnection with exponential backoff, the connection status in the UI. — *phase 7: keepalive, reconnection, a delay of 1 s → 1 min, the state in the UI live through `server_status`.*
- [x] One SSH connection per server, multiplexing sessions (SFTP + exec) for all projects of that server.

**Reading logs:**

- [x] Listing the log folder and `stat` over SFTP, reading the rest of files from the saved offset (`ReadAt`). The state (file, size, offset, mtime) is kept in SQLite, so that nothing is read again after a restart.
- [ ] Live tail mode. ❓ *Polling SFTP every N seconds or `exec` of `inotifywait`/`tail -F` on the remote side — section 10.18.* — *partly: polling SFTP every 2 s (section 10.18, phase 7); no speed-up through `inotifywait`.*
- [ ] Traffic compression (SSH compression) and a speed limit per server, so that big logs do not clog the link.
- [x] Logs inside Docker on a remote server: read through the volume path on the host. ❓ *Or through `docker exec … cat` — section 10.21.* — *per section 10.21 — the volume path on the host: it is a regular log folder, no separate code needed.*

**Control and operations:**

- [x] ✅ Turning the debugger on and off on a remote project by writing `dws_debugger.json` over SFTP (atomically: a temporary file + `rename`). This is the main way of control (section 10.22); HTTP is only the legacy mode (section 7.4a). — *a temporary file next to it + `posix-rename`, permissions are kept.*
- [x] Clearing and deleting logs on the remote side only inside the log folder of the project (checked through `realpath`), with a confirmation in the UI. — *phase 7: only files of the log folder are deleted, by name (no paths, links or directories); clearing is a common button of the project with a confirmation, through the source of the server; forbidden on "read only" servers and without the delete permission.*
- [x] A "read only" mode for a server: it forbids deleting and changing the config, for example for stage. — *server permissions (phase 5), set when an SSH server is added (phase 7).*
- [x] Source previews over SFTP (section 5.1) with a cache by `mtime`. — *v0.5, without a cache: only a window around the line is read, from a file of up to 1 MB.*
- [x] Connection test: host reachability, permissions on the log folder, presence of `Debugger.class.php`/the config, the PHP version (through `exec php -v`, if allowed). — *phase 7: step by step — settings, connection, SFTP, for every project the log folder (reading; writing — with a trial file, except on "read only" servers), the library config; `php -v`. The config and PHP give warnings and do not fail the test.*
- [x] IDE links for remote projects: a mapping "remote path → local copy" (`/var/www/app` → `~/work/app`), and without one — a link for VS Code Remote-SSH (`vscode://vscode-remote/ssh-remote+<host>/<path>:<line>`). — *phase 7: and for Cursor; PhpStorm and Sublime get no link without a local copy.*

### 7.4a Legacy HTTP mode (optional)

For servers without SSH access (old shared hosting, someone else's infrastructure) the viewer can work through the HTTP mode of the PHP library (section 4.1).

- [x] Server type `http`: the base URL of the project + a token (kept in the system secret store, like SSH passwords, section 10.19). — *phase 7: until the system store (section 10.19) the token lives in `secrets.json` next to the database (permissions 0600), write-only in the API; one server — one application and one project.*
- [x] `HTTPPullSource`: polling `list` and `read` with an offset on a schedule, live tail — by frequent polling; `enable`/`disable`/`clear` from the UI. — *phase 7: `source.HTTP`, polling every 2 s; it passes the common `sourcetest` contract and a test against the real library under `php -S`.*
- [x] HTTPS only (HTTP — with an explicit warning in the UI), certificate verification, timeouts, a request rate limit on the viewer side. — *phase 7: the certificate is always verified, a 30 s timeout, requests no more often than once per 50 ms, after a 429 — a pause by `Retry-After`; `http://` — a warning in the form and in the test.*
- [ ] Features not available in this mode (source previews, project discovery) are hidden in the UI with an explanation. — *partly: an HTTP project has the log folder and the config hidden; source previews and discovery do not exist in any mode yet (phases 8, 10).*
- ✅ The viewer does not support the old HTTP API (`?debug_msg=…`) (section 10.36): for the HTTP mode library 2.x is installed on the server.

### 7.5 Several servers and projects

**Data model:**

✅ **Decided (section 10.20):** many servers, each with its own projects. A project belongs to exactly one server; cluster projects (one project on several nodes) are not supported.

```
Server (local | ssh | http)   — connection type, parameters, environment label, permissions (read-only and so on)
 └─ Project                    — name, root, log folder, path mapping for the IDE, editor, retention
     └─ Request                — one log file / one HTTP or CLI request
         └─ Message            — an event inside a request
```

**Tasks:**

- [x] Tables `servers`, `projects` (`server_id`), `requests`, `messages` (without Hungarian notation, section 4.4). Instead of the former `tprojects` with four paths — `projects` with one root and a log folder on its server + a mapping for the IDE. **The schema and the `Source` interface are designed right in phase 3 and implemented in the MVP** (even if the MVP has only the local server), so that SSH and several servers do not require reworking the schema and the API.
- [ ] A config file with the list of servers and projects (YAML) that can be edited by hand and through the UI, and imported and exported (without secrets). — *partly (phase 8): export and import of YAML without secrets (API `GET/POST /config`, the "Export / import" window): the import adds what is missing, a file with an error or a secret is rejected entirely; the token of an HTTP server is set afterwards in the servers window. The source of truth is the database, not the file: a change of the file is applied by importing it.*
- [x] Discovering projects on a server: looking for `dws_debugger.json` or log folders by a mask in given roots (like the current `scan_projects`, but for any server). — *phase 8: "Find projects" at a local and an SSH server: roots, up to 4 levels and 5000 folders, no links, `vendor`/`node_modules`/`.git` are skipped; paths — by the rules of the library (with v1 keys); already added ones are marked; checked on a real OpenSSH too.*
- [ ] Parallel connectors: a goroutine per server, a common limit of simultaneous imports, isolation of errors (one unreachable server does not get in the way of the others). — *partly: an import loop per project (phase 5); no common limit.*
- [x] UI: a "servers → projects" tree in the sidebar; statuses; colored environment labels (`local`, `dev`, `stage`, `prod`) and a warning on destructive actions on `stage`/`prod`. — *phase 8: groups by server on the projects panel, the connection state live, the environment stripe of the selected project; on `stage`/`prod` clearing and deleting require typing the project name.*
- [x] Combined views: "all errors across all projects", "latest requests everywhere", a filter by server and environment. — *phase 8: a common feed with "only with errors", the feed of a server on a click on its name (`#/servers/N`), an environment filter, "project · server" in the row.*
- [x] Handling clock skew between servers (important for combined feeds): keep the server time and the time of receipt, show a warning on a big difference. — *phase 8: `received_at` next to `ts`; the clock offset of the server (SSH — `date` on connect, HTTP — the `Date` header) in the `clock_offset_ms` of the status; more than 2 s — a warning at the server in the UI.*
- [ ] Retention and quotas at the server and project level (how much to keep in the local database and how much to leave on the remote side).
- [x] Permissions at the server level: read-only, allowing deletion, allowing control of the debugger.

### 7.6 Optional: a Go agent next to the PHP project

- [x] A light agent `wapp-debugger agent` for cases where SSH is not available (cloud containers, Kubernetes pods): it reads the log folder of the PHP library and sends events to the central server (`HTTPSource`). With SSH available the agent is not needed. — *v0.8: `wapp-debugger agent --logs-dir … --push …` (the token — `WAPP_DEBUGGER_PUSH_TOKEN`): new v2 lines as they are written, a retry after a failure, after a restart — again without duplicates; v1 files are skipped.*

### 7.7 Transition

1. [ ] Specifications: the log format, the API contract v1, the database schema with `servers/projects` (phase 3).
2. [ ] Walking skeleton (phase 4), then the MVP for the local source: projects, import, requests, messages, variables (phase 5).
3. [ ] The SSH source (section 7.4, phase 7), then the features of several servers (section 7.5, phase 8).
4. [ ] Data migration: ❓ *carry over the old `db/dbfile.db` or just re-import the JSONL files (simpler, recommended) — section 10.10.*
5. [ ] Delete the PHP server (`index.php`, `ajax.php`, `controllers/`, `models/`, `lib.php`, `database.php`, `rb.php`, `sse.php`, `docker/nginx`, `docker/php-workspace`). The repository keeps `client-php/` (the debugger), `server/` (Go) and `ui/` (Svelte).

### 7.8 Desktop version (Wails)

✅ **Decided:** a desktop is needed. It is built with **Wails**: the Go core (`internal/*`) and the Svelte UI (section 6) are shared with the server version, there is no separate UI code for the desktop.

**Structure:**

- [x] Layout: `cmd/wapp-debugger` (the server/CLI) and `desktop/` (the Wails application) on top of the same `internal/*` packages.
- [x] ✅ **Wails v3** (section 10.26), checked in the walking skeleton (phase 4). The version is pinned exactly (`go.mod`), updates come as separate PRs after a CI run on three OSes. All code that depends on Wails is kept in `desktop/` as a thin layer (the window, the tray, events, deep links), so that a fallback to v2 touches only it.
- [x] **Criteria for falling back to v2** (checked in phase 4): it does not build or does not pass the smoke test on one of the OSes in CI; serving REST through the asset handler or the live stream does not work (section 10.27); blocking bugs of the tray, the frameless window, single instance or deep links without a workaround. On fallback — v2 + a separate tray library.
- [x] UI ↔ core communication: the same REST API as in the server version. The Go HTTP handler is served through the Wails asset handler (`AssetOptions.Handler` in v3), so no network port is opened. The live stream goes through Wails events if SSE through the asset handler is unreliable. In the UI this hides behind one `transport` interface (HTTP+SSE in the browser, Wails in the desktop). ❓ *To be confirmed by a prototype — section 10.27.*
- [ ] A "desktop + browser access" mode: optionally start the same server on `127.0.0.1:<port>` with a token, to open the UI in a browser as well.
- [ ] Data and config in the standard OS directories (`os.UserConfigDir`, `os.UserCacheDir`); a config format shared with the server version, import and export. — *partly: the database is in `os.UserConfigDir`; no shared config file.*
- [ ] SSH in the desktop: ssh-agent on Linux/macOS (`SSH_AUTH_SOCK`), on Windows — the OpenSSH agent (named pipe) and Pageant; secrets in the system store (Keychain / Credential Manager / Secret Service), section 10.19.

**Desktop features:**

- [ ] Our own window title bar (frameless) in the style of the current theme: a continuation of section 6.3 about replacing the default UI. On macOS — the native "traffic lights", on Windows/Linux — our own buttons.
- [x] A tray/menu bar icon with a counter of new errors, a quick menu (pause live, latest errors, open the window). — *phase 9: a counter of unseen requests with errors (reset when the window gets focus), the menu: open, the latest 5 errors with navigation, quit; closing the window hides it in the tray (a setting). No live pause in the menu.*
- [x] System notifications about new errors (with consent in the settings, with a filter by project and level). — *phase 9: about a request — once, at its first error (or warning — by a setting), in the chosen projects; a burst — one summary "N more"; a click opens the request; the language — as in the UI.*
- [x] Deep links `wapp-debugger://open?project=…&request=…` from a browser, an IDE or a chat. — *partly: parsing the link (only `open` with numeric ids) and navigation in the UI, from arguments (Windows, Linux) and the macOS event (phase 9); the scheme is registered in `.desktop` (Linux), the HKCU registry (Windows), `Info.plist` (macOS).*
- [x] A single instance of the application (a second launch activates the window and passes the deep link). — *phase 9: Wails SingleInstance, a link from the second launch opens in the first window.*
- [ ] Autostart at login (an option), remembering the size and position of the window and the panel layout. — *partly: autostart (a setting) and the size, position and maximized state of the window (phase 9); the panel layout is not remembered.*
- [ ] A native application menu (macOS) with shortcuts matching the UI. — *partly: the standard macOS menus (application, edit, view, window) with their shortcuts (phase 9); the UI has no shortcuts of its own there yet.*
- [ ] Dragging JSONL/ZIP from a file manager into the window to import.
- [ ] Auto-update. ❓ *Whether it is needed and how — section 10.29.* — *per section 10.29: no self-update; a notice about a new version (phase 9): GitHub Releases at most once a day, in the desktop — by a setting (on), in the server — by the `--update-check` flag; a link to the release in the UI header, in the desktop it opens in the system browser.*

**Build and delivery:**

- [x] `make desktop-dev`, `make desktop-build`, `make desktop-package` (section 3.1). — *all three; `make icons` renders the icons from SVG.*
- [x] Installers: macOS `.dmg` (universal: arm64 + amd64), Windows NSIS `.exe` (WebView2 — a bootstrapper in the installer), Linux AppImage and `.deb` (depends on `libwebkit2gtk`). ❓ *Whether `.rpm`, Flatpak, a Homebrew cask, winget, Scoop are needed — section 10.32.* — *phase 9 (`make desktop-package`, `scripts/desktop-package.sh`): Linux — `.deb` (nfpm through `wails3 tool package`) and AppImage, Windows — an NSIS `setup.exe` for a user without administrator rights (a WebView2 check with a download link, no bundled bootstrapper), macOS — a universal `.app` in a `.dmg` (ad-hoc signed); everywhere — the `wapp-debugger://` scheme. Channels — GitHub Releases only (section 10.32).*
- [ ] Signing and notarization (macOS Developer ID + notarytool, Windows Authenticode) in the release workflow (section 3.2). ❓ *section 10.28.*
- [x] Tests: unit tests of the `transport` layer, a smoke test launching the built application in CI on three OSes (it starts, serves the UI, closes).

---

## 8. Order of work: phases and milestones

### 8.1 What was wrong with the previous version of the order

1. **Wasted work on code that will be deleted.** Stages 1–4 included fixing and optimizing the old PHP server and EasyUI (RedBean, indexes, import, tags, dead code, Dockerfile), although both are being removed. Now the old code gets only hotfixes (🔥), the rest ↪ are requirements for Go.
2. **Double work in the PHP library.** Bugs were fixed in the old file, and then the same code was rewritten. Now the old file gets only cheap fixes of fatal errors and vulnerabilities.
3. **HTTP control was built twice** (tokens in the old code and in the new one). Now the hotfix has only "off by default + a required token", and a full optional HTTP mode is built once in the new library.
4. **The schema for several servers came last (7c)**, although it defines the database and the API. Now the schema is designed in phase 3 together with the contracts.
5. **The main technical risks were checked at the end**: embedding the UI into Go, SSE inside Wails (section 10.27), building on three OSes, CGO/WebKit on Linux. Now the walking skeleton in phase 4 removes them.
6. **No milestones and no MVP.** The release workflow came after everything, the first release — who knows when. Now every phase has a version.
7. **The monorepo was not a separate step**, although the Makefile and CI depend on it; section 4.1 had `packages/client`, other places `client-php/`.
8. **Doing "Go and UI at the same time"** is realistic only with two developers. ✅ There is one developer (section 10.34), so the phases go one after another, the MVP — as vertical slices, and PHP library v2 was moved after the MVP.

### 8.2 Phases

The phase number = the order of work (one developer).

| Phase | Content | Result / milestone | Estimate |
|---|---|---|---|
| **0. Decisions** | ✅ Done: 10.6, 10.13, 10.17, 10.20, 10.22, 10.23, 10.26, 10.34 | Recorded decisions | — |
| **1. Hotfix of the current version** ✅ the code is ready, checked on PHP 7.4 and 8.4; left to commit and tag | 🔥 from sections 1.1, 2.1, 2.2 (see section 9); syntax no newer than already used in the file, no PHP 8 functions | the `legacy-hotfix` tag (not SemVer, not to be confused with versions of the new product): the library is safe to keep in projects | 1–2 days |
| **2. Repository foundation** ✅ | Monorepo (`legacy/` for the old code), `.env`, v1 fixtures, a Makefile skeleton (`help`, `doctor`, `lint`, `test`, `ci`) and `ci.yml`, CONTRIBUTING with the naming rules (section 4.4) | Empty but buildable `client-php/`, `server/`, `ui/`, `desktop/`; green CI | 2 days |
| **3. Specifications** ✅ | JSON Schema of formats v1 and v2 (section 4.2), OpenAPI v1 — including the legacy HTTP mode of the library (section 4.1), the database schema `servers/projects/requests/messages` and the `Source` interface (section 7.5), type generation (`make generate`) | `spec/` — one contract for PHP, Go, UI | 2–3 days |
| **4. Walking skeleton** | Go `serve`: the v1 parser on fixtures, 2–3 endpoints + SSE; Svelte: one screen with the list of requests, tokens and a light/dark theme switch; a **Wails v3** shell with the same UI and a check of the live stream (fallback criteria — section 7.8); CI builds server+ui+desktop on three OSes; `make release-snapshot` | The end-to-end path "log → Go → UI → desktop → artifact" works; decisions 10.26/10.27 confirmed or a fallback to v2 | 3–5 days |
| **5. Viewer MVP** | Steps in section 8.2d: English as the base language and i18n; the v2 parser and import from an offset; fsnotify; request and message details in the API and UI; projects and control of the debugger; search and filters; secure by default; removing `legacy/`; `release.yml` | **v0.1.0** — the first release of the new product; the old server and EasyUI removed | 15–22 days |
| **6. PHP library v2** ✅ the code is ready; publishing 2.0.0 waits for the owner | A package for PHP 7.4+ (section 4.1), format v2 (`snake_case`), naming (section 4.4), deprecated wrappers (section 10.23), masking, error/exception handler, rotation, PSR-3/Monolog of both lines, the optional HTTP mode | `client-php` **2.0.0** on Packagist (through a split, section 3.2); the viewer shows v2 logs right away | 6–8 days |
| **7. SSH** ✅ tag v0.2.0 | Section 7.4: connection, known_hosts, SFTP reading from an offset, live tail, control of the debugger through the config, read-only; then the legacy HTTP mode (section 7.4a) | **v0.2.0** | 5–8 days |
| **8. Several servers** ✅ tag v0.3.0 | Section 7.5: the "servers → projects" tree, project discovery, combined feeds, environments and warnings, clock skew, permissions (the schema has been there since phase 3) | **v0.3.0** | 3–4 days |
| **9. Full desktop** ✅ the code is ready; tag v0.4.0 — after confirmation | Section 7.8: tray, notifications, deep links, single instance, autostart, installers, signing (per 10.28), a notice about a new version | **v0.4.0** | 4–6 days |
| **10. Growth** | UI features (sections 5.1, 6.5: timeline, SQL, comparison, palette, theme editor); the HTTP receiver + `HttpWriter` + the JS client (section 5.2); the agent (section 7.6); framework integrations and the SQL/Guzzle helpers (section 5.3) | iterative minor versions → **v1.0.0** | iterative |

**Criteria of v1.0.0:** a stable format v2 and API v1 (only backward compatible changes after that), SSH, the legacy HTTP mode and several servers, a desktop on three OSes, documentation on installation and integration, no known vulnerabilities.

### 8.2a Results of phase 2

Done (branch `phase-2-foundation`):

- **Monorepo**: the old server, EasyUI and library v1 were moved into `legacy/` without changing their behavior (checked by running `php -S`); `.env` was removed, there is `legacy/.env.example`, the personal mount was replaced with `PROJECTS_PATH`.
- **`client-php/`**: a Composer package (PHP ≥ 7.4), PHPUnit 9.6, PHPStan 2 (level 8, `phpVersion: 70400`), PHP-CS-Fixer (`@PSR12`, `@PHP74Migration`). The first code through TDD — the name check `tools/bin/check-naming` (a ban on Hungarian notation, section 4.4): 22 tests, green on PHP 7.4 and 8.4; it finds 466 names in `legacy/Debugger.class.php`.
- **`server/`**: a Go module (Go 1.26), the CLI `wapp-debugger version|help` and the `internal/version` package through TDD, golangci-lint v2.
- **`ui/`**: Vite 8 + Svelte 5 + TypeScript 6, Vitest + Testing Library, ESLint (with a rule against Hungarian notation, `snake_case` allowed for properties), Prettier, `svelte-check`; through TDD — the theme choice (`resolveTheme`, `parseThemePreference`, section 6.4). The build goes to `server/web/dist`. Components are written with `<script lang="ts">`, otherwise `svelte-check` treats them as JS without types.
- **`spec/fixtures/v1/`**: 7 logs written by the real library v1 on PHP 7.4 (web requests through `php -S`, CLI, AJAX, a long URL, a killed process, a truncated line, a version header corrupted before the hotfix) and `manifest.json` with reference properties; the generator is `make fixtures-v1`.
- **Makefile**: `help`, `doctor`, `tools` (golangci-lint v2.15.0, actionlint v1.7.12 in `.tools/bin`), `deps`, `lint`, `test`, `build`, `ci`, targets per component, `php74-test`, `fixtures-v1`. `make ci` passes locally.
- **CI** (`.github/workflows/ci.yml`): PHP 7.4–8.4, PHP lint, Go, UI, build; actions pinned by SHA, `permissions: contents: read`; Dependabot for actions, Go, npm, Composer. Checked with `actionlint`; not run on GitHub yet (the branch is not pushed).
- **Documentation**: the README for the new layout, CONTRIBUTING (TDD, naming, commits, glossary).

Not done in phase 2 and moved:

- `desktop/` — only a README: the Wails v3 skeleton needs the CLI and system libraries installed, it is done in phase 4 together with checking the fallback criteria.
- A ban on Hungarian notation for Go is not made a separate rule: Go has none of it in the old code, and `revive` checks idiomatic names.
- Node in CI is 24 LTS (`ui/.nvmrc`); locally it works on 22 too.

### 8.2b Results of phase 3

Done (branch `phase-3-specs`), all through tests in `server/internal/spec`, `server/internal/store`, `server/internal/source`, `server/internal/api`, `ui/src/lib/api`:

- **Format v1** — JSON Schema of the header and the message, describing the format as it is; checked on 7 real fixtures and `manifest.json`.
- **Format v2** — [`spec/logformat/README.md`](spec/logformat/README.md) and a strict JSON Schema of the events `request_start`/`message`/`timer`/`error`/`request_end`; stream rules (`seq` without gaps, one `request_id`, `request_start` first, nothing after `request_end`, `ts` does not decrease); 4 reference requests, 19 invalid events, 6 invalid streams; a v1 → v2 mapping table.
- **Database schema** — SQLite migrations (`STRICT`, JSON checks, cascades, FTS5 synchronized by triggers), embedded into the binary; [`spec/db/README.md`](spec/db/README.md).
- **`source.Source`** — the interface of a log source, a shared contract test suite `sourcetest.Run` and an in-memory implementation.
- **OpenAPI** — [`spec/openapi.yaml`](spec/openapi.yaml) (servers, projects, the combined request feed, messages, SSE, settings) and [`spec/library-http.yaml`](spec/library-http.yaml); tests check `snake_case`, `operationId`, Problem Details, examples and the required token.
- **Code generation** — `make generate` / `make generate-check` and a CI job: Go (oapi-codegen as a `go tool`) and TypeScript (openapi-typescript); tests catch stale generated code and a mismatch between the UI themes and the contract.

Decisions made along the way (cheap to change while no code relies on them):

- v2: one file per request `<ts>-<request_id>.jsonl`, append only; `request_id` is a ULID; levels `error`/`warning`/`info`/`message`/`debug` (`debug` added for PSR-3); the schema is strict, the viewer tolerates extra fields when reading.
- v1: `sMessage` can be any scalar — that is how the library writes it, the schema allows it.
- The HTTP mode of the library — separate paths `/__wapp_debugger/v1/…` instead of `?debug_msg=` parameters (clear routing and a description in OpenAPI), a token of ≥ 32 characters; without a token the library does not answer.
- The viewer API: OpenAPI 3.0.3 (oapi-codegen does not fully support 3.1); the `Bearer` token is optional on `127.0.0.1` and in the desktop.
- Search: FTS5 `unicode61` — by words and prefixes (`медлен*`), case and Cyrillic; a URL substring — through `LIKE` (section 10.37).
- openapi-typescript runs through `npx` with a pinned version: its peer dependency is TypeScript 5, while the UI is on TypeScript 6.
- `make server-test` runs with `-count=1`: the `go test` cache does not notice changes in `spec/` (outside the Go module) and reported a stale "ok".

### 8.2d Phase 5 step by step

Every step is a vertical slice through TDD (Go + UI + tests), separate commits in the `phase-5-mvp` branch.

1. **English as the base language, i18n.** Translate into English the code comments, the descriptions in `spec/` (JSON Schema, OpenAPI — code is generated from them), the READMEs of fixtures, the CLI messages; README and CONTRIBUTING in English + `README.ru.md`; commits — in English. In the UI — `en` (default) and `ru` dictionaries, a language choice in the header, saved like the theme; a test that the keys of the dictionaries match. The license — per the decision of section 10.39.
2. **The v2 parser** on the `spec/fixtures/v2` fixtures (invalid streams included) and **import from an offset**: v2 is read on from where it stopped, v1 is re-read entirely; `log_files.read_offset`, handling a truncated file (`ErrOffsetBeyondEnd`).
3. **fsnotify** for a local folder with fallback polling and debounce, so that the file is not re-read on every line.
4. **Request and message details**: `getRequest`, `listMessages` (levels, type, tag, cursor), `getMessage` (context, backtrace, `previous`); `messages_added` in the stream. UI: the layout "feed → request → message", the context tree with types, the backtrace with IDE links.
5. **Projects and control**: `listServers`/`listProjects`/`createProject`/`updateProject`/`deleteProject` for the local server, `getDebuggerState`/`setDebuggerState`, `clearProjectLogs`, the counter of unprocessed files. UI: the projects sidebar, the project page, our own confirmation dialogs.
6. **Search and filters**: `q` (FTS5 over messages and context, a URL substring), `has_errors`, `sapi`, `since/until`; the filter line in the UI.
7. **Secure by default**: `127.0.0.1`, an optional `Bearer` token when not listening on localhost, refusing to start on `0.0.0.0` without a token; headers (CSP, `X-Content-Type-Options`), request size limits.
8. **Wrap-up**: delete `legacy/` and `docker/`, documentation on installing and connecting library v1 to the new viewer, `release.yml` (`v*` tags → GoReleaser, Docker in GHCR, unsigned desktop builds), the tag **v0.1.0** — after explicit confirmation.

### 8.2e Results of phase 5

Steps 1–8 are done (branch `phase-5-mvp`), except the tag **v0.1.0**: it is set after explicit confirmation and a decision on the license (section 10.39).

- **Language and i18n**: code, comments, specifications, the CLI, API errors and commits — in English; the UI — `en`/`ru` dictionaries with a check that keys match and with plural forms; `README.md` + `README.ru.md`.
- **Import**: the v2 parser on fixtures (invalid streams included), v2 is read on from the offset in chunks, v1 is re-read entirely; fsnotify with debounce and fallback polling; the project manager — a separate loop per project.
- **API and UI**: feed → request → message (the context as a tree, the backtrace with IDE links), child AJAX requests; projects (a panel, an add/settings dialog, a debugger switch, clearing logs and deleting through our own confirmation dialogs), the feed filter by project in the URL.
- **Search**: `q` over words of messages and context (FTS5, prefixes, all words; input is escaped word by word) or a URL substring; `has_errors`, `sapi`, `server_id`, `environment`, `since/until`; search over messages inside a request. In the UI — a search line with a 250 ms delay, Web/CLI, "Only with errors".
- **Security**: a non-loopback address without a token — the start is refused; the token through `WAPP_DEBUGGER_TOKEN` (or `--token`), `Bearer` or an HttpOnly cookie after signing in with `?token=`; without a token — only loopback host names (DNS rebinding); a write with a foreign `Origin` — 403; a strict CSP without inline scripts (the theme script moved to a file); a request body ≤ 1 MiB (413). The desktop has no network port and so none of these checks.
- **Wrap-up**: the old viewer, EasyUI and docker were removed; library v1 moved to `client-php/v1/Debugger.class.php` (projects use it until `client-php` 2.0); the README was rewritten for the new viewer with a screenshot; `release.yml` — the whole CI, a GoReleaser draft release, unsigned desktop archives for three OSes, an image in GHCR (distroless, 15 MB, multi-arch without emulation).

Decisions along the way:

- Library v1 lives in `client-php/v1/` outside the linters (Hungarian notation stays there until the `@deprecated` wrappers of phase 6).
- The project root without `--root` is the nearest parent of the log folder with `dws_debugger.json` (up to three levels): for `tmp/logs` the project used to be called `tmp`.
- A live feed with filters adds a new request only if it surely matches; text search is checked by the server, so with an active `q` new requests show up after the feed reloads.
- The release is a draft: it is published by hand after a review. The Docker image listens on all interfaces and therefore requires a token; `/data` is writable for any `--user`.
- GoReleaser is installed as a ready binary with a `checksums.txt` check: `go install` required Go 1.27 and failed on network errors of `sum.golang.org`.
- The desktop self-test in CI: GTK4 under Xvfb without a GPU raises a window in ~28 s — rendering through cairo/software GL and waiting up to 2 minutes; on Ubuntu 24.04 the AppArmor restriction on user namespaces is lifted for the WebKit sandbox.

The license is MIT (section 10.39), the `LICENSE` file is added. CI of phase 4 on GitHub is green after the fixes. Left before v0.1.0: green CI of the `phase-5-mvp` branch and the tag on confirmation.

### 8.2f Phase 6 step by step

Branch `phase-6-client-php`, all through TDD (PHPUnit on PHP 7.4 and 8.x). The output of the library in tests is checked by the same JSON Schema v2 (`spec/logformat/v2`) and stream rules as in Go.

1. **The write core.** `Level`, ULID, clock, `Config` (an array or `dws_debugger.json`, `snake_case` keys), `JsonlFileWriter` (append only, permissions `0664`/`0775` configurable), `Debugger`: `request_start` → `message` → `request_end`. A disabled debugger costs one `is_file()`.
2. **Serialization and masking.** `Serializer` (limits of depth, string length and number of items; objects with properties including private ones, recursion, resources, closures, exceptions) and `Masker` (the list of keys is configurable).
3. **Source, trace, timers, tags, parent.** `source` and a short `trace` with paths from the project root, a full one — by an option or for errors; `timer` for `start/stop`; the `tags` field; `parent_request_id` from the `X-Wapp-Debugger-Parent` header (web) or the `WAPP_DEBUGGER_PARENT` variable, which the library sets for child processes — to be added to the specification.
4. **Errors and the outcome of the request.** `ErrorHandler`: PHP errors, exceptions (with `previous`), fatal errors in shutdown; `request_end` with the response or exit code and peak memory; the application's previous handlers are not lost.
5. **Compatibility.** `__d`/`__log`/`__start`/`__stop` forever; `fn*` and `DWS_*` — `@deprecated` (one `E_USER_DEPRECATED` per request); old config keys (`bIsOnFlag`, `sMessagesFilesPath`…) are read with a warning. Including without Composer — one `require` of the autoloader.
6. **Rotation and retention**: limits on the number of files, the total size and age; deleting the oldest by means of PHP, without `exec`, no more often than once per N requests.
7. **PSR-3 and Monolog**: a `LoggerInterface` adapter for `psr/log` 1.x and 2.x/3.x, a handler for Monolog 2 and 3; a CI matrix over the dependency lines.
8. **HTTP mode** by `spec/library-http.yaml`: off without a token (≥ 32 characters), `hash_equals`, only a header, an optional IP allowlist.
9. **Wrap-up**: v2 fixtures written by the real library and a Go test of their import; the library README (en, ru) and migration from v1; the split of `client-php/` into a separate repository and publishing on Packagist — these need the owner's actions (the repository, registering the package, a token); the `client-php` tag **2.0.0** — after confirmation.

### 8.2g Results of phase 6

Steps 1–9 are done (branch `phase-6-client-php`), except publishing: the split into a separate repository and Packagist need the owner's actions, the `client-php` 2.0.0 tag — after confirmation.

- **Library 2.x** (`client-php/src`, PHP 7.4+, no required dependencies): format v2, `Debugger::boot()` (a disabled one costs one `is_file()`), serialization with limits and masking by the v1 rule, the call site and a short trace, timers on a monotonic clock, tags, parent requests, PHP errors, exceptions and fatal errors with the outcome of the request, rotation, PSR-3 (psr/log 1–3), Monolog 2 and 3, the HTTP mode by `spec/library-http.yaml`.
- **Compatibility with v1**: `__d`/`__log`/`__start`/`__stop` behave as before (the timer level by duration, a stack of unnamed timers), `fn*` — `@deprecated`, old config keys are read with a warning in the log; including without Composer — `src/autoload.php`.
- **Checks**: 141 PHPUnit tests; every written line is checked by JSON Schema v2 and the stream rules; process tests on real PHP (a warning, an uncaught exception, running out of memory, a project "as on v1", the HTTP mode behind `php -S`); `make php74-test` — real PHP 7.4 with dependencies for 7.4; in CI all lines of psr/log and Monolog. The `spec/fixtures/v2-library` fixtures are written by the library itself (`make fixtures-v2-library`), and Go tests check and import them.
- **Publishing**: `release-php.yml` (the tag `client-php/vX.Y.Z` → the whole CI → a check against `Debugger::VERSION` → split → the tag `vX.Y.Z` in `hightemp/wapp-php-debugger`), the package's `LICENSE` and `.gitattributes`; the library README (en, ru) with the move from v1.

Found and fixed along the way:

- An uncaught exception cannot be rethrown from the handler: PHP 8 calls the same handler again, and it went into endless recursion. Now the output of PHP is reproduced byte for byte (honoring `log_errors`, `display_errors`, `html_errors`), the exit code is 255.
- After running out of memory the handler set the limit to "current usage + 16 MB" and could lower it; now the limit is only raised.
- The debugger switch in the viewer changed `bIsOnFlag` even when the config had `enabled`, which library 2.x listens to; now `enabled` wins, as in the library.
- The former `make php74-test` ran PHP 7.4 on a `vendor/` built for 8.4; now the dependencies are installed for 7.4 in a copy, through the host proxy.
- The Composer `lock` file is not committed (it is a library): every PHP version in CI gets its own line of dependencies; psr/log 2 is pinned in the PHP 8.1 job.

Decisions along the way:

- Superglobals at start — `$_GET`, `$_POST`, `$_FILES`; `$_SERVER`, the session and cookies — only through `log_globals` (section 2.1).
- Errors outside `error_reporting` and those suppressed with `@` are not written — just like PHP itself does.
- Permissions in `dws_debugger.json` are octal strings (`"0664"`): JSON has no octal numbers.
- The HTTP mode serves and deletes only files with log names — stricter than the contract, which forbids only `/`.
- PHPStan does not analyze the Monolog handlers: with `phpVersion` 7.4 it does not see the classes of Monolog 3, and Monolog 2 and 3 cannot be checked in one run; tests on both lines cover them.

Left for 2.0.0: the `hightemp/wapp-php-debugger` repository, a deploy key in the `release` environment, registration on Packagist; then `Debugger::VERSION = '2.0.0'` and the tag `client-php/v2.0.0`.

### 8.2h Phase 7 step by step

Branch `phase-7-ssh`, all through TDD. The open questions of the phase are settled by the recommendations of section 10: live tail — polling SFTP (10.18), secrets — only ssh-agent and keys, passwords through a keyring — later and when needed (10.19), Docker — the volume path on the host (10.21).

1. ✅ **Install in one line** (the owner's request): a one-file build of `client-php` (`make php-bundle` → `dist/wapp-debugger.php`, a test run on the built file, the file in the release); `install.sh` for the server (OS and architecture, the latest release, a `checksums.txt` check); `curl` one-liners in the README (en, ru).
2. ✅ **SSH client**: `~/.ssh/config` (aliases, `ProxyJump`), ssh-agent and keys, host key verification by `known_hosts` with confirmation of a new fingerprint (TOFU), keepalive and reconnection with a delay, one connection per server. Tests — against an SSH/SFTP server inside the test.
3. ✅ **`SSHSource`** — an implementation of `source.Source` over SFTP (the common `sourcetest` contract): listing, reading from an offset, file truncation, writing `dws_debugger.json` atomically (a temporary file + `rename`), deleting only inside the log folder; live tail by polling.
4. ✅ **Servers API and UI**: `createServer`/`deleteServer`/`checkServer` for `ssh`, fingerprint confirmation, the connection status, the "read only" mode; projects on an SSH server.
5. ✅ **Connection test**: the host, permissions on the log folder, the library config, the PHP version (`exec php -v`, if allowed); IDE links for remote projects (a mapping to a local copy or VS Code Remote-SSH).
6. ✅ **Legacy HTTP mode** (section 7.4a): the `http` server type, `HTTPPullSource` by `spec/library-http.yaml`, HTTPS only (HTTP — with a warning).
7. ✅ **Wrap-up**: integration tests against a real `openssh-server` in CI, documentation, the tag **v0.2.0** — after confirmation (waiting).

### 8.2i Results of phase 7

Steps 1–7 are done (branch `phase-7-ssh`); the tag **v0.2.0** — after confirmation.

- **Install in one line**: `make php-bundle` builds the library into one file `dist/wapp-debugger.php` (the whole test suite passes on it too, PHP 7.4 included), `install.sh` installs the viewer for the OS and architecture with a `checksums.txt` check; both files are in the release, the `curl` one-liners are in the README (en, ru).
- **SSH** (`internal/sshconn`): `~/.ssh/config` with aliases, `IdentitiesOnly` and `ProxyJump` chains; keys from `IdentityFile`, then ssh-agent; the user's `known_hosts` is only read, new keys are confirmed in the UI by fingerprint and written into our own file, a changed key is refused; one connection per server, keepalive, reconnection with a pause of 1 s → 1 min, the state in the UI live.
- **`source.SSH`**: SFTP, reading on from an offset, polling every 2 s, deleting only files in the log folder, the switch — a temporary file and `posix-rename` keeping permissions.
- **Servers in the API and UI**: adding, a step-by-step test (connection, the log folder of every project — reading and writing, the library config, `php -v`), key confirmation, deletion, "read only"; projects on remote servers; IDE links through VS Code / Cursor Remote-SSH when there is no local copy.
- **HTTP mode** (section 7.4a): an `http` server with the application address and a token, `source.HTTP` by `spec/library-http.yaml`, a test by the status of the library, a warning for `http://`.
- **Checks**: an SSH server inside Go tests (sftp, exec, jump, keepalive, a dropped connection); `make ssh-e2e` — the whole user path against a real OpenSSH in Docker, in CI too; the HTTP source — the common contract on a test server and a test against the real library under `php -S`.

Found while checking on a real OpenSSH and fixed:

- With many keys in ssh-agent the server cut the login off ("Too many authentication failures"): the key from `IdentityFile` was offered last. Now it is first, as in OpenSSH, and `IdentitiesOnly` is honored.
- The wrong host key type was requested: the hint in the UI sent you to compare ED25519 with an ECDSA key, and a key of another type in `known_hosts` looked like "the key changed". Now the known types are requested first, ED25519 for new hosts, and the UI names the type and the file.
- Escape and Tab in windows did not work when focus left a button that disappeared; now there is a stack of windows and handling on `window`.
- Without `--logs-dir` there was no local server, and saving a local project silently did nothing.

Decisions along the way:

- Polling SFTP every 2 s (section 10.18), only ssh-agent and keys (section 10.19), Docker — through the volume path on the host (section 10.21).
- The token of an HTTP server, until the system secret store (section 10.19), lives in `secrets.json` (0600) next to the database; it is not in the database or in API responses.
- An HTTP server is one application and one project: the library has one log folder; a bare 404 without `problem+json` means "the library did not answer" (a wrong address or token), not "no such file".
- Optional test steps (the library config, PHP, HTTPS) are warnings: without a config the debugger is just off, `exec` may be forbidden.

Left for later: entering a key passphrase and SSH passwords through the system store (section 10.19), `inotifywait` instead of polling, SSH compression and a speed limit, source previews over SFTP.

### 8.2j Phase 8 step by step

Branch `phase-8-servers` (from `phase-7-ssh`), all through TDD. No open questions of section 10 block the phase.

1. ✅ **Playwright e2e** (from section 6.2): the build, the viewer on fixtures, the scenario "project → request → message → variables" and a check that the screens render in all built-in themes; the **e2e** job in CI with a report and screenshots on failure (section 3.2).
2. ✅ **The "servers → projects" tree** in the sidebar: the connection status of a server, colored environment labels (`local`, `dev`, `stage`, `prod`) and the environment stripe of the selected project; on `stage`/`prod` clearing and deleting require typing the project name, the debugger switch (both ways) — a confirmation.
3. ✅ **Combined feeds and filters**: a filter by server and environment in the API and UI, "all errors across all projects", the server and project in the feed row.
4. ✅ **Clock skew**: estimating the clock difference of a server on connect (SSH — `date +%s.%N` through exec, without exec the offset is unknown; HTTP — the `Date` header), `clock_offset_ms` in the server status, a warning in the UI when the difference is over 2 s.
5. ✅ **Project discovery**: looking for `dws_debugger.json` in given roots on a local and an SSH server (depth and number of directories are limited, links are not followed), a list of what was found with one-click adding.
6. **Import without overload**: a common limit of simultaneous imports, isolation of errors (one unreachable server does not slow down the others), retention in the local database by the project's `retention_days`/`retention_max_bytes`.
7. ✅ **Virtualized lists** (from section 6.2): the request feed, the messages of a request, the tree of variables.
8. ✅ **Config of servers and projects**: export and import of YAML without secrets (host keys and tokens are left out).
9. ✅ **Wrap-up**: documentation, the results of the phase, the tag **v0.3.0** — after confirmation (waiting).

### 8.2k Results of phase 8

Steps 1–9 are done (branch `phase-8-servers`); the tag **v0.3.0** — after confirmation.

- **Playwright e2e** (`make e2e`, the **e2e** job in CI): the built viewer on fixtures written by the real library; the path "project → request → message → variables", a link to a message, three built-in themes with snapshots, a request with 3000 messages.
- **Servers → projects**: groups on the projects panel with the environment and the connection state live, the feed of a server (`#/servers/N`), an environment filter and "project · server" in the common feed; on `stage`/`prod` clearing and deleting — by typing the project name, the debugger switch — with a confirmation.
- **Clock skew**: `date +%s.%N` over SSH on connect, the `Date` header in the HTTP mode; `clock_offset_ms` in the server status, a warning when it is over 2 s.
- **Project discovery** (`POST /servers/{id}/discover`) on a local and an SSH server by `dws_debugger.json`, with limits and without links; paths — by the rules of the library, v1 keys included.
- **Import without overload**: no more than 4 passes at once, a pass — no longer than 2 min, the same source error is written once; the project's retention in the viewer database (days, size) every 10 minutes.
- **Large volumes**: our own `VirtualList` for the feed and messages, all messages of a request in pages, big nodes of the tree of variables — 200 at a time.
- **Export and import** of servers and projects as YAML without secrets; the token of an HTTP server is set in the servers window.
- **Library**: `allowed_envs` (off outside the listed environments), the "no dependencies" rule with tests (`PackageTest`, running the build under `php -n`).

Found and fixed along the way:

- A request showed only its first 500 messages, the rest were silently lost; now all are loaded.
- The retention of a project could not be removed through the API: `null` in PATCH meant "do not change"; now the `retention` object replaces both limits.
- The message panel was not a named region: screen readers (and the e2e test) could not find it.
- The build of the library under `php -n` stayed silent on PHP 7.4 in CI: json is a loadable module there; `ext-json` is declared in `composer.json`, the check loads only it.
- `make ssh-e2e` failed on the image registry rate limit — `docker pull` with retries.

Decisions along the way:

- The virtual list is our own, without dependencies: rows of one height, measured by the first row; the tree of variables is not virtualized but shown in parts (the library limits depth and number of items anyway).
- The config file is export and import, the source of truth stays in the database; the import only adds.
- Confirmation by typing the name — for clearing and deleting on `stage`/`prod`; for the debugger switch — a plain confirmation both ways.

Left for later: quotas at the server level, an environment frame around the whole window, comparing theme snapshots with references.

### 8.2l Phase 9 step by step

Branch `phase-9-desktop` (from `phase-8-servers`), all through TDD. The open questions of the phase — by the recommendations of section 10: no signing yet, a first-launch guide for the release (10.28); no auto-update, only a notice about a new version from GitHub Releases (10.29); channels — GitHub Releases (10.32). The logic is in `internal/desktop` (tested without a window), Wails is a thin layer in `cmd/wapp-debugger-desktop`.

1. ✅ **Desktop settings**: `GET/PUT /api/v1/desktop/settings` — notifications (on, minimum level, projects), autostart, update check; a settings window in the UI only in the desktop.
2. ✅ **Notifications and the error counter**: from the event stream — new requests with errors by the settings filter; grouping (no more than one notification in a few seconds, "N more"); Wails system notifications; a counter in the tray.
3. ✅ **Tray**: an icon with a counter of new errors, a menu (open the window, the latest errors — going to them, quit); closing the window hides it in the tray.
4. ✅ **Deep links and a single instance**: `wapp-debugger://open?project=…&request=…&message=…` → a UI route; a second launch passes the link to the open window; the scheme is registered in the installers.
5. ✅ **Window**: the size and position are remembered; autostart at login (an option); the application menu on macOS with shortcuts.
6. ✅ **Notice about a new version**: a check of GitHub Releases once a day (in the desktop — on by default, can be turned off; in the server — by a flag), a banner in the UI.
7. ✅ **Installers**: Linux `.deb` and AppImage, Windows NSIS `.exe`, macOS `.dmg` (universal) — unsigned, with the `wapp-debugger://` scheme; in `release.yml`.
8. ✅ **Wrap-up**: documentation (including the first launch without signing), the results of the phase, the tag **v0.4.0** — after confirmation (waiting).

### 8.2m Results of phase 9

Steps 1–8 are done (branch `phase-9-desktop`); the tag **v0.4.0** — after confirmation. Releases are published when everything is ready (the owner's decision): until then tags produce drafts.

- **Desktop settings** (`GET/PUT /api/v1/desktop/settings`, the "Settings" window only in the desktop): notifications (level, projects), autostart, tray, update check; the language of notifications follows the UI.
- **Notifications and tray**: `Alerter` in `internal/desktop` — about a request once, at its first error (or warning), a burst — one summary, a counter of unseen ones in the tray reset on window focus, a menu of the latest errors; closing the window hides the application in the tray.
- **Deep links** `wapp-debugger://open?…` (only `open` with numeric ids), a single instance (a link from a second launch goes to the open window), the window remembers its size and position, the standard macOS menus.
- **New version**: GitHub Releases at most once a day, without blocking while the request runs; in the desktop — by a setting, in the server — by the `--update-check` flag (by default the server does not go to the network); a link in the header, in the desktop — in the system browser.
- **Installers** (`make desktop-package`): Linux `.deb` and AppImage, Windows NSIS `setup.exe` without administrator rights, macOS universal `.app` in a `.dmg`; the `wapp-debugger://` scheme everywhere; a new icon from SVG (`make icons`). CI builds them on three OSes, runs the AppImage and keeps them for a week as artifacts.

Found and fixed along the way:

- The macOS notification service does not start for an application without a bundle identifier and crashed the self-test: on macOS notifications come only from an `.app`.
- e2e caught an error in the browser console from a `404` on `/updates` with the check off: the endpoint answers `200` with `enabled: false`.
- `go.sum` lacked the Windows notifications module; the name `wails3.exe` in the Makefile was expanded before `GOEXE` was defined.

Decisions along the way:

- No signing (10.28) — a first-launch guide in the README; no self-update (10.29); channels — GitHub Releases (10.32).
- WebView2 is not bundled into the installer: Windows 11 has it, on Windows 10 the installer offers the download page.
- The AppImage carries GTK and WebKit (~80 MB), the `.deb` relies on the system `libgtk-4-1` and `libwebkitgtk-6.0-4`.

Left for later: the "desktop + browser access" mode, SSH passwords in the system store (10.19), dragging logs into the window, our own window title bar, the panel layout, signing and notarization.

### 8.2n Phase 10 by iterations

The phase is iterative: every iteration is its own branch, tests (TDD, e2e for screens) and a minor version after confirmation. The order is by usefulness for debugging legacy projects; the library stays without dependencies (section 4.1).

1. ✅ **v0.5 — inspecting a request** (branch `phase-10-request-detail`): request tabs (overview, a timeline/waterfall of messages and timers, SQL with duration and N+1 repeats, superglobals); library helpers — SQL through a PDO wrapper and HTTP requests (a Guzzle-compatible middleware without a dependency on Guzzle); a source preview around a backtrace line with highlighting (only inside the project root, locally and over SFTP); the tree of variables — search and copying of the path and the value.
2. ✅ **v0.6 — speed of work**: the `Ctrl+K` command palette, keyboard shortcuts and help on `?`, a filter line with syntax (`level:error tag:sql duration>500 method:POST status:5xx url:/api/*`) and autocompletion, live pause, pinning and notes on requests, toast notifications.
3. ✅ **v0.7 — sharing and comparing**: copying a request as cURL, replaying with `XDEBUG_SESSION`, export and import of a request (JSONL/ZIP), comparing two requests (a diff of variables), several requests in tabs.
4. ✅ **v0.8 — new sources**: receiving logs over HTTP right into the viewer (the library's `HttpWriter`, a token), the JS client `debugger.js` for the front end tied to the `request_id` of the page, importing third-party logs (`php_errors.log`, Laravel, nginx), the agent `wapp-debugger agent` (section 7.6).
5. ✅ **v0.9 — appearance** (branch `phase-10-v09`): UI settings on the server (`/api/v1/settings`), the accent color, density, fonts, the environment frame, a theme editor with import and export, `prefers-contrast`, Stylelint on colors, the component showcase, visual snapshots of themes. — *The interface language is kept in the settings too. Stylelint pulls in `braces` with an open advisory and no fixed version (a dev dependency only): to be kept in mind for `npm audit` in v1.0.*
6. ✅ **v1.0 — readiness** (the criteria below; branch `phase-10-v10`): integrations and instructions (Laravel, Symfony, WordPress, Bitrix), freezing format v2 and API v1, CI — CodeQL, `govulncheck`, `npm audit`, attestations, SBOM, fuzz, `paths` filters; release notes, issue/PR templates, publishing `client-php` 2.0.0 and the releases. — *The freeze: `spec/frozen/v1.0`, `make spec-compat` (oasdiff) and a test of v2 logs. Left to the owner: publishing `client-php` 2.0.0 (a repository for the split, a deploy key, Packagist), publishing the draft releases, protecting `main`, turning on private vulnerability reporting (`SECURITY.md` and the issue templates link to it). What is postponed is in `ROADMAP.md`.*

### 8.3 Dependencies and timing

```
1 (hotfix, independent)
2 ── 3 ── 4 ── 5 (v0.1) ── 6 (client-php 2.0) ── 7 (v0.2) ── 8 (v0.3) ── 9 (v0.4) ── 10 (→ v1.0)
```

- In total up to v0.4 ≈ **37–56 working days** of one person. The most uncertain part is phase 5; if it drags on, the first thing cut is the UI scope beyond the required minimum (section 6.6), not the tests.
- PHP library v2 (phase 6) comes after the MVP: the viewer already reads v1, and a live UI shows at once whether the new format is convenient. The v2 specification is ready since phase 3, so the v2 parser in Go is written and tested in advance.
- The urgent phase 1 does not depend on the others and is done first: old projects keep working on the old library until `client-php` 2.0 is out.
- The Makefile and CI are not a separate phase: every phase adds its own targets and jobs (sections 3.1, 3.2).
- Every milestone is a tag and a release through `release.yml` (since phase 5), so that a new version can be used right away, without waiting for v1.0.

---

## 9. Hotfix of the current version (phase 1) — ✅ done

Only fixes that remove fatal errors and easily exploitable vulnerabilities in the code in use now. Syntax and functions — no newer than PHP 7.0 (no PHP 8 functions), checked with `php -l` on 7.4 and 8.4.

**PHP library (`Debugger.class.php`):**

- [x] Default constants: `DA_ROOT_DIR = __DIR__`, `DA_FLAG = is_file(dws_debugger.json)` (the debugger is active only when the config exists; it is turned on by the `bIsOnFlag` flag in it). The constants can be defined before including the file. The guard against including twice moved to `DA_LOADED`.
- [x] `fastcgi_finish_request()` only if the function exists (`php -S`, mod_php no longer crash in shutdown); `posix_*` only when the extension is there; `$_SESSION`, `HTTP_HOST`, `argv`, backtrace frames — through `isset`, without warnings.
- [x] `iArraySizeLimit` and `__log()` do not crash on scalars; `__log([...])` with one item does not reach for a nonexistent `[0]`.
- [x] The header of a log file is always exactly 1000 bytes and valid JSON: long `sURL`/`argv` are shortened, in the worst case only the service fields stay. The first messages are no longer overwritten.
- [x] HTTP commands (`?debug_msg=…`, `?save_debug_message`) are off until the `DA_HTTP_TOKEN` constant is set (≥ 16 characters); the token is in the `X-Debugger-Token` header, compared with `hash_equals`. Without a token the request just goes to the application.
- [x] `get_file` — only files inside `sProjectRootPath` (`realpath`); `?debug_msg=<file>` — only a file name without paths.
- [x] `set_config`/`start` take from the request only `bIsOnFlag`, `iPageSize`, `iArraySizeLimit`, `iLogsSizeLimit` (with type casting); paths change only by editing the config on the server.
- [x] All `exec` calls are removed: the folder size, the disk space, deleting old logs and `clear` — in PHP. `clear` does not follow symbolic links. Cleanup by `iLogsSizeLimit` (in KB, as `du -s` had it before) deletes the oldest files until the size is below the limit; it runs only when the debugger is on.
- [x] `save_debug_message`: `json_decode(..., true)`, field checks, the right order of arguments.
- [x] Masking values by keys (`password`, `pass`, `pwd`, `secret`, `token`, `api_key`, `app_key`, `private_key`, `access_key`, `authorization`, `php_auth_pw`, `cookie`, `dsn`, `csrf`) in all logged arrays, `$_SERVER` included (env passwords, `HTTP_COOKIE`, `HTTP_AUTHORIZATION`). `$_SESSION` and `$_COOKIE` are written whole only with `"bLogSessionAndCookies": true` in the config.
- ↪ A known limitation (it was there before the hotfix too): including the file again through `require`/`include` gives the fatal "Cannot redeclare", because PHP declares the functions of a file before the check at its start runs. Include it with `require_once`. In library 2.x Composer autoloading solves it.
- ↪ Only array keys are masked: secrets inside objects, in the message text and in the query string of `sURL` are not masked (section 4.1, `Masker`).

**Old viewing server:**

- [x] `s_message` is escaped on the server (`controllers/debug_messages.php`) before links to files are inserted; in all formatters and columns of EasyUI (`debug_files.js`, `debug_messages.js`, `debug_message_info.js`, `projects.js`) data from logs is escaped through `fnEscapeHtml`/`fnEscapeFormatter` (`static/app/modules/lib.js`), the request URL, the backtrace and the tree of variables included.
- [x] `Access-Control-Allow-Origin: *` was removed from `docker/nginx/conf.d/vhost.conf`.
- [x] Listen only on `127.0.0.1`: `bin_start_server.sh`, the port forwarding in `docker-compose.yaml`, the example in the README.

**Check:** a set of scenarios on PHP 7.4 (Docker `php:7.4-cli`) and 8.4: without a config, logging with secrets and a 1500-character URL, HTTP without a token, with a wrong and a short token, with the right token (`status`, `get_file` with `../`, `set_config` with paths, `clear` with a symbolic link, `debug_msg=../…`), cleanup by the limit, a real web request through `php -S`. Escaping in the old UI was checked only syntactically (`node --check`), not in a browser.

**Migration for existing projects:** if HTTP commands were used — define `DA_HTTP_TOKEN` before including the file and pass the `X-Debugger-Token` header. If `$_SESSION`/`$_COOKIE` are needed in the logs — add `"bLogSessionAndCookies": true` to `dws_debugger.json`.

---

## 10. ❓ Decisions that need clarification

Every question lists the options and a recommendation. ✅ — decided.

**When to decide:**
- ✅ **phase 0:** decided — 10.6, 10.13, 10.17, 10.20, 10.22, 10.23, 10.26, 10.34, 10.36;
- **phase 2:** ✅ 10.14 (monorepo, section 3), 10.35 (the history is not rewritten);
- **phase 4 (after the prototype):** confirming 10.26, 10.27, 10.7;
- **before release v0.1 (phase 5):** 10.4, 10.11, 10.24, 10.25, 10.30, 10.33;
- **before v0.4 (phase 9):** 10.28, 10.29, 10.32;
- the rest — along the way.

| # | Question | Options | Recommendation |
|---|--------|----------|--------------|
| 10.1 | ✅ What to rewrite in Go | — | **Decided:** the viewer in Go, the debugger stays in PHP |
| 10.2 | ✅ A Go agent next to the project | — | **Decided:** the main way is SSH (section 7.4), the agent — only for environments without SSH (section 7.6) |
| 10.3 | ✅ Svelte: SPA or SvelteKit | — | **Decided:** Svelte + Vite, SSR is not needed, it is simpler to embed into the Go binary |
| 10.4 | Components: write everything ourselves or a headless library? | Our own from scratch / Bits UI, Melt UI + our styles | Headless for the complex ones (Dialog, Combobox, Menu, Tooltip — focus and ARIA), the rest our own |
| 10.5 | How deep to replace the "default" UI? | Only dialogs, menus, tooltips, select / plus scrollbars, Ctrl+F, right click | Dialogs, menus, tooltips, select — a must; scrollbars — only styled through CSS (no JS emulation); catch Ctrl+F only when a list has focus |
| 10.6 | ✅ SQLite driver in Go | — | **Decided:** `modernc.org/sqlite` — no CGO, an easy cross-build of the server, FTS5; replacing the driver is isolated in `internal/store` (section 7.3) |
| 10.7 | Realtime transport | SSE / WebSocket | SSE: the stream is one-way, simpler, and works through proxies |
| 10.8 | ✅ The old EasyUI interface | — | **Decided:** the UI is rewritten from scratch, EasyUI is removed entirely, without a `/legacy` mode |
| 10.9 | ✅ Desktop version | — | **Decided:** needed, on Wails (Go + Svelte), section 7.8 |
| 10.10 | Carrying over existing data | Migrating `db/dbfile.db` / re-importing JSONL | Re-import: the database is derived data. The plan has no separate migration step |
| 10.11 | ✅ Languages | — | **Decided:** English is the base (code, comments, commits, CLI, API errors, documentation, the default UI language), Russian is the second UI language and the translation of user documentation (`README.ru.md` and so on) |
| 10.12 | Compatibility of the log format | Support v1 forever / a compatibility period | Read v1 forever, write only v2 |
| 10.13 | ✅ Minimum PHP version for the library | — | **Decided:** PHP 7.4+ (old projects). The consequences are in section 4.1: no enum/match/union types, two lines of `psr/log` and Monolog, a CI matrix 7.4–8.4 |
| 10.14 | Repository layout | A monorepo (`client-php/`, `server/`, `desktop/`, `ui/`, `spec/`) / separate repositories | A monorepo (the layout in section 3); publish the PHP library on Packagist through a split |
| 10.15 | Multi-user mode | Only local / a team with authorization | Local + a token. With SSH servers the viewing server stays personal (the user's keys). A shared team server is a separate task |
| 10.16 | ✅ Delivery | — | **Decided:** binaries and a Docker image through GoReleaser, desktop installers, the PHP library through Packagist; all from GitHub Actions (section 3.2) |
| 10.17 | ✅ Naming style in the log format, the config and the API | — | **Decided:** `snake_case`; DTOs in TS stay `snake_case` (section 4.4) |
| 10.18 | Live tail over SSH | Polling SFTP (`stat` every 1–2 s) / `exec inotifywait` / `exec tail -F` | Polling SFTP by default (needs nothing on the server); `inotifywait`, when installed, as a speed-up |
| 10.19 | SSH secrets | Only agent and keys / plus passwords in the OS keyring / passwords in the config | Agent and keys; passwords — only through the OS keyring (`go-keyring`), never in the config or the database |
| 10.20 | ✅ What "several project servers" means | — | **Decided:** many servers, each with its own projects; a project belongs to one server (section 7.5) |
| 10.21 | Logs in Docker on a remote server | Through the volume path on the host / `docker exec` / an agent in the container | The volume path on the host; `docker exec` as a fallback (needs docker permissions) |
| 10.22 | ✅ Controlling the debugger | — | **Decided:** the main way is SSH (writing the config over SFTP); the HTTP mode is optional for legacy environments: off by default, only with a token, without access to sources and paths (sections 4.1, 7.4a) |
| 10.23 | ✅ Compatibility of the PHP API when renaming | — | **Decided:** `__d`/`__log`/`__start`/`__stop` forever; `fn*` and `DWS_*` — `@deprecated` until the next major version (3.0) |
| 10.24 | The set of built-in themes | Only light / dark / system / high contrast / plus popular presets (Solarized, Nord, Dracula, Monokai…) | Four basic ones in the first version; add presets later, when the tokens settle — they are just token files |
| 10.25 | ✅ Where to keep the theme choice and custom themes | — | **Done as recommended (v0.9):** in the Go server settings (the same in every browser and in the desktop version), `localStorage` is a cache for an instant load |
| 10.26 | ✅ Wails version | — | **Decided:** try v3 in the walking skeleton (phase 4) with explicit criteria for falling back to v2 (section 7.8) |
| 10.27 | UI ↔ core transport in the desktop | REST through `AssetServer.Handler` + Wails events / Wails bindings (generated methods) / a local HTTP port | REST through the asset handler: one API for the web and the desktop; prototype the live stream before the main work starts |
| 10.28 | **Signing the applications** | No signing (Gatekeeper/SmartScreen warnings) / macOS signing ($99/year Apple Developer) / plus a Windows certificate | The first versions unsigned with a launch guide; macOS signing — as soon as there are users outside the team. A decision on the budget for certificates is needed |
| 10.29 | Desktop auto-update | None (only a notice about a new version) / self-update from GitHub Releases / through package managers | A notice about a new version through the GitHub Releases API in the first version; self-update — after signing appears |
| 10.30 | ✅ Release process | — | **Decided (v1.0):** a manual tag after confirmation + the GoReleaser changelog from Conventional Commits; the draft release is published by hand |
| 10.31 | Building on Windows without make | Only Git Bash/WSL / a duplicate `make.ps1` / Taskfile instead of make | Git Bash/WSL; CI on Windows calls the same targets through bash |
| 10.32 | Desktop distribution channels | Only GitHub Releases / plus a Homebrew cask, winget, Scoop, AUR, Flatpak | GitHub Releases in the first version; a Homebrew cask and winget next (GoReleaser automates them) |
| 10.33 | Versioning of components | One version for everything (server, desktop, UI, PHP library) / a separate version of the PHP library | The server, desktop and UI — one version; the PHP library — its own SemVer, compatibility by the log format version (`v`) |
| 10.34 | ✅ How many people work on the project | — | **Decided:** one; phases one after another, as vertical slices (section 8) |
| 10.35 | ✅ The email in the git history (`.env`) | — | **Decided:** the history is not rewritten; the file was removed from the repository in phase 2 |
| 10.36 | ✅ Reading the legacy library (after the hotfix) over HTTP | — | **Decided:** only library 2.x; the viewer does not support the old `?debug_msg=…` commands |
| 10.37 | ✅ Full-text search: by words or by substring | — | **Decided:** FTS5 `unicode61` (words and prefixes); a second `trigram` table over `message` — only if phase 5 needs searching for parts of names and paths |
| 10.39 | **License** | MIT / Apache-2.0 / GPL-3.0 / other | ✅ **MIT** (decided): the `LICENSE` file in the root, `"license": "MIT"` in `client-php/composer.json`, release archives include `LICENSE` |
| 10.40 | ✅ The language of this plan | Keep it Russian (a working document) / translate into English | Keep it Russian until v0.1.0; for contributors by the release — a short `ROADMAP.md` in English based on the plan — *v1.0: `ROADMAP.md` in English.* — *after 1.0: the plan is translated: `IMPROVEMENT_PLAN.md` in English, `IMPROVEMENT_PLAN.ru.md` is the Russian original; both are kept in sync.* |
