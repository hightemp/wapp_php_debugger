# v2 log fixtures

Written by hand from the specification [`../../logformat/README.md`](../../logformat/README.md) before library 2.x existed: they are the reference for both the writer (`client-php`) and the parser (`server`).

| File | What it shows |
|---|---|
| `web-request.jsonl` | GET: all levels, tags, context with a masked key, `source` and `trace`, a timer, a `php_error` |
| `ajax-child-request.jsonl` | AJAX POST with `parent_request_id` pointing to `web-request`, a masked password |
| `cli-exception.jsonl` | CLI: a slow timer, an exception with `previous`, a fatal error, `exit_code` |
| `killed-without-end.jsonl` | No `request_end`: the process was killed or is still running |
| `invalid/*.json` | One event each that the schema must reject; the file name says why |
| `invalid-streams/*.jsonl` | Valid events that break the stream rules |

Some messages are in Russian on purpose: the fixtures exercise non-ASCII text.
