# Log formats

| Version | Written by | Schema | Fixtures |
|---|---|---|---|
| v1 | `client-php/v1/Debugger.class.php` (library 1.x) | [`v1/header.schema.json`](v1/header.schema.json), [`v1/message.schema.json`](v1/message.schema.json) | [`../fixtures/v1`](../fixtures/v1) |
| v2 | `client-php` 2.x | [`v2/event.schema.json`](v2/event.schema.json) | [`../fixtures/v2`](../fixtures/v2) |

The viewer always reads both versions; the new library writes v2 only. The version of a file is detected from its first line: every v2 event has `"v": 2`.

The schemas are checked by the tests in `server/internal/spec` (`make server-test`).

## v1

One file per request, named after the request start time in milliseconds. The first line is a header padded with spaces to 1000 bytes and rewritten in place; then one line per message. Known defects a reader must survive:

- a header longer than 1000 bytes (library before `legacy-hotfix`) overwrites the beginning of the first message, so that line does not parse as JSON;
- a process killed before shutdown leaves `bFinished: false` and no final messages;
- a truncated last line;
- `sMessage` can be any scalar, and `oVars` is an object even for lists.

## v2

One file per request: `<start ts>-<request_id>.jsonl`, append-only (never rewritten, so a reader can safely continue from a saved offset, including over SFTP). Every line is a self-contained event:

| `type` | When | Main fields |
|---|---|---|
| `request_start` | first event | `sapi` (`web`/`cli`), `lang`, `library_version`; for web — `method`, `url`; for cli — `argv`; `parent_request_id` for AJAX and child processes |
| `message` | `__d()`, PSR-3, Monolog, SQL and HTTP helpers | `level`, `message`, `tags`, `context`, `duration_ms`, `source`, `trace` |
| `timer` | `__stop()` | `name`, `duration_ms`, `level` |
| `error` | PHP errors, exceptions, fatal errors | `kind` (`php_error`/`exception`/`fatal`), `level`, `message`, `error_type` or `exception_class`, `previous` |
| `request_end` | shutdown | `duration_ms`, `status_code` (web) or `exit_code` (cli), `memory_peak_bytes` |

Fields of every event: `v` (2), `type`, `request_id` (ULID), `seq`, `ts` (ms since the epoch, server clock).

**Stream rules** (checked by tests, not by the schema):

1. The first event is `request_start`, and there is only one.
2. All events of a file share one `request_id`.
3. `seq` starts at 1 and grows by 1 without gaps — a gap means a lost line.
4. `ts` never decreases.
5. At most one `request_end`, and only as the last event. No `request_end` means the request is still running or the process was killed.

**Parent request.** A web request takes `parent_request_id` from the `X-Wapp-Debugger-Parent` header (the page passes its own `request_id` to its AJAX calls); a CLI process takes it from the `WAPP_DEBUGGER_PARENT` environment variable, which the library sets to the current `request_id`, so processes started from a request (exec, proc_open, queued jobs run in place) link to it automatically. A value that is not a ULID is ignored.

**Durations** (`duration_ms` of `timer`, `request_end` and of a `message` about a timed operation) are measured on a monotonic clock and may be fractional; `ts` is the wall clock. An event with a duration is written when the operation ends: it spans `[ts - duration_ms, ts]` on the request timeline.

**SQL and HTTP calls** are messages with a tag and a duration, so the viewer can list and time them:

- `tags: ["sql"]` — `message` is the query text (with placeholders), `context.params` its parameters, `context.rows` the affected or returned rows when known;
- `tags: ["http"]` — `message` is `METHOD url` of an outgoing request, `context.status` the response status, `context.error` the failure.

**Levels:** `error`, `warning`, `info`, `message`, `debug`. PSR-3 levels map as follows: `emergency`, `alert`, `critical`, `error` → `error`; `warning` → `warning`; `notice`, `info` → `info`; `debug` → `debug`.

**Names** are `snake_case`. Paths in `source` and `trace` are relative to the project root. The library masks secrets before writing (`"***"`).

### Mapping v1 → v2

| v1 | v2 |
|---|---|
| header `iRequestTimestamp` | `ts` of `request_start`; the v1 request key is the file name (real files are named after `iRequestTimestamp`, but only the name is guaranteed unique in the logs directory) |
| `bIsCli` | `sapi` |
| `sURL`, `bIsAjax`, `aArgv`, `sIP`, `sServerIP`, `sServerName` | `url`, `is_ajax`, `argv`, `ip`, `server_ip`, `server_name` |
| `sUser`, `sSysUser`, `sSession`, `sLang` | `process_user`, `app_user`, `session_id`, `lang` |
| `bFinished`, `iCode`, `iTotalTime` | presence of `request_end`, `status_code`, `duration_ms` |
| `sType` | `level` (`functions_debug` and unknown types → `message`) |
| `sMessage` | `message` (scalars are converted to a string) |
| `#timediff #name` tags in the text | `tags`; the message `#timediff #name N мс` becomes a `timer` event |
| `oVars` | `context` |
| `oBackTrace` (`file`, `line`, `function`) | `source` |
| `aBackTrace` (`class`, `type`) | `trace` (`class`, `call_type`) |
| `iTimestamp` | `ts` |
