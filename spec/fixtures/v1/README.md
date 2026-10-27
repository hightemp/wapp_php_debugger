# v1 log fixtures

**English** · [Русский](README.ru.md)

Logs in the current format (`client-php/v1/Debugger.class.php`), written by the real library on PHP 7.4. The v1 parser tests in Go and the UI e2e tests use them.

v1 format: the first line is the request header with a fixed length of 1000 bytes (JSON padded with spaces, rewritten in place when the request finishes), then one JSON line per message (`sType`, `sMessage`, `oVars`, `oBackTrace`, `aBackTrace`, `iTimestamp`).

`manifest.json` holds the reference properties of every file (header validity, `bFinished`, `bIsCli`, `bIsAjax`, number of valid and broken lines, levels). `generator/manifest.php` computes it, and it is reviewed by hand after regeneration: the parser tests compare their results with it.

| File | Scenario |
|---|---|
| `web-get.log` | GET through `php -S` (SAPI cli-server): all levels, a timer, `__log`, `E_USER_ERROR`, found/not found fields |
| `web-ajax-post.log` | AJAX POST, the password in `$_POST` is masked |
| `cli.log` | CLI with arguments |
| `long-url.log` | URL longer than 1000 bytes: `sURL` is shortened to 300 bytes, the header is exactly 1000 bytes |
| `killed.log` | Process killed with `SIGKILL` before shutdown: `bFinished=false`, no final messages |
| `truncated-last-line.log` | The last line is cut off in the middle of a write |
| `pre-hotfix-long-header.log` | Library before the hotfix (`ecf8962`): a 2714-byte header overwrote the start of the first message; secrets are not masked (the values are synthetic) |

The log messages are partly in Russian on purpose: the fixtures exercise non-ASCII text.

Regenerate (Docker required): `make fixtures-v1`. Timestamps and sizes change, the scenarios do not.
