# Viewer database schema

SQLite (`modernc.org/sqlite`, no CGO). The source of truth is the migrations in [`server/internal/store/migrations/`](../../server/internal/store/migrations): they are embedded into the binary and applied when the database is opened. The constraints are pinned down by `server/internal/store/schema_test.go`.

```
servers ─┬─ projects ─┬─ log_files ── requests ── messages ── messages_fts (FTS5)
         │            └──────────────── requests (log_file_id = NULL: HTTP ingest)
settings
```

| Table | Stores | Key constraints |
|---|---|---|
| `servers` | Connections: `local`, `ssh`, `http`; environment `local`/`dev`/`stage`/`prod`; permissions `read_only`, `allow_delete`, `allow_control`; `config` is JSON without secrets | unique name |
| `projects` | A project on one server: root, logs directory, local copy for IDE links, editor, retention | name and logs directory unique per server |
| `log_files` | Read state of a file: size, `mtime`, `read_offset`, format version, status | `read_offset ≤ size`; path unique per project |
| `requests` | A request: `request_start`/`request_end` data (or the v1 header), message and error counters, `received_at` for clock skew | `request_key` unique per project |
| `messages` | `message`, `timer` and `error` events | `seq` unique per request; `context` is a JSON object |
| `messages_fts` | Full-text index over `message` and `context` (unicode61, case-insensitive, Cyrillic included) | kept in sync by triggers, including cascading deletes |
| `settings` | UI and viewer settings | the value is JSON |

Deleting a server, a project or a log file cascades to everything below it. Times are milliseconds since the epoch (`INTEGER`), booleans are `0`/`1`, all tables are `STRICT`.
