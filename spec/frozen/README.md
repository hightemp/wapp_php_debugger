# Frozen contracts

**English** · [Русский](README.ru.md)

At 1.0 the contracts the parts of wapp-debugger talk through are frozen: from here on they change
only in a backward compatible way, so a library, an agent or a script written against 1.0 keeps
working with every 1.x viewer.

`v1.0/` is a copy of the contracts as they were at 1.0. It is never edited.

| Contract | Frozen copy | Checked by |
|---|---|---|
| Viewer API v1 (`/api/v1`) | [`v1.0/openapi.yaml`](v1.0/openapi.yaml) | `make spec-compat` (oasdiff: no breaking change, no warning) |
| Receiver of pushed logs (`/ingest/v2`) | [`v1.0/ingest.yaml`](v1.0/ingest.yaml) | `make spec-compat` |
| HTTP mode of the library 2.x | [`v1.0/library-http.yaml`](v1.0/library-http.yaml) | `make spec-compat` |
| Log format v2 | [`v1.0/logformat/`](v1.0/logformat/event.schema.json), [`v1.0/fixtures/`](v1.0/fixtures) | `server/internal/spec/frozen_test.go`: every frozen log is valid for the current schema and read by the viewer without problems |

## What may change

- New endpoints, new optional query parameters and request fields, new response fields.
- New event fields and types in log format v2 (the viewer ignores fields it does not know), new
  enum values where readers are told to expect them.
- Descriptions, examples, limits that only widen.

## What may not

- Removing or renaming an endpoint, a field, an enum value or an event type; making something
  optional required; narrowing a type or a limit; changing what a field means.

Such a change waits for a new major version: API v2 under `/api/v2` next to v1, log format v3 with
`"v": 3` next to v2. The new frozen copy then goes to `v2.0/`.
