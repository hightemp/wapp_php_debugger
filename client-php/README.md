# wapp-php-debugger: PHP library 2.x

[Русская версия](README.ru.md)

The debugger side of [wapp_php_debugger](../README.md): it writes the events of every request (messages, timers, errors, the outcome) into JSONL logs, one file per request, and the viewer shows them live. PHP 7.4 and newer, no required dependencies.

## Install

With Composer:

```sh
composer require --dev hightemp/wapp-php-debugger
```

The package is not on Packagist yet. Until it is, point Composer at a checkout of this repository:

```sh
composer config repositories.wapp-debugger path /path/to/wapp_php_debugger/client-php
composer require --dev hightemp/wapp-php-debugger:@dev
```

Without Composer, copy `client-php/src` into the project and require its autoloader:

```php
require __DIR__ . '/lib/wapp-debugger/autoload.php';
```

## Start

Call `boot()` as early as possible, in the front controller or the CLI entry point:

```php
WappDebugger\Debugger::boot(__DIR__ . '/dws_debugger.json');
```

and put `dws_debugger.json` into the project root:

```json
{
    "enabled": true,
    "logs_dir": "tmp/logs"
}
```

Without the file, or with `"enabled": false`, the library does nothing: a request costs one `is_file()` and a small JSON read. On, `boot()` starts the request, registers the error handlers (the handlers of the application keep working and PHP still prints errors as usual) and finishes the request at shutdown with the HTTP status or the exit code. The viewer turns `enabled` on and off for you.

## Logging

```php
__d(DWS_INFO, 'Order created', ['order' => $order]);   // level, message, named variables
$total = __log($price) * $quantity;                      // logs a value and returns it
__start('db'); /* ... */ __stop('db');                   // timer: info, warning over 1 s, error over 2 s
```

The same through the object:

```php
use WappDebugger\Debugger;
use WappDebugger\Level;

$debugger = Debugger::instance();
$debugger->log(Level::DEBUG, 'SELECT * FROM items WHERE id = ?', ['params' => [$id]], ['sql']); // tags
$debugger->exception($caught);          // with its trace and previous exceptions
$debugger->startTimer('render');
$debugger->stopTimer('render');
```

Levels: `error`, `warning`, `info`, `message` (neutral), `debug`. Every event knows where it came from (file and line, relative to the project root) and, by default, the last 10 frames of the call stack.

Values are made safe for the log: depth, items and string length are limited, objects are shown with all their properties, recursion is cut. Keys that look like secrets (`password`, `token`, `api_key`, `authorization`, `cookie`, `dsn`… — the same rule as library v1) are written as `***`.

### Parent requests

A page and its AJAX calls, or a job and the processes it starts, are linked in the viewer:

- web: send the page's id in the `X-Wapp-Debugger-Parent` header of its AJAX calls; the page gets it from `Debugger::instance()->requestId()`;
- CLI: nothing to do — the library sets `WAPP_DEBUGGER_PARENT` for processes started from a request.

## PSR-3 and Monolog

```php
$logger = new WappDebugger\Bridge\PsrLogger();                     // Psr\Log\LoggerInterface: psr/log 1, 2 or 3
$monolog->pushHandler(new WappDebugger\Bridge\MonologHandler());   // Monolog 2 or 3, whichever is installed
```

PSR-3 levels map onto the five levels above (`critical`, `alert`, `emergency` → `error`, `notice` → `info`); an exception in the `exception` key of the context is written with its trace. Laravel: add the handler to a channel in `config/logging.php` (`'driver' => 'monolog', 'handler' => WappDebugger\Bridge\MonologHandler::class`). Symfony: register it as a Monolog service handler.

## Configuration

`dws_debugger.json`; relative paths are resolved against its directory. File and directory modes are octal strings: JSON has no octal numbers.

| Key | Default | Meaning |
|---|---|---|
| `enabled` | `false` | Write logs |
| `project_root` | directory of the file | Paths in the logs are relative to it |
| `logs_dir` | `<project_root>/tmp/logs` | Where the log files go; must be writable by PHP |
| `file_mode`, `dir_mode` | `"0664"`, `"0775"` | Modes of new files and directories (the viewer often runs as another user) |
| `trace_frames` | `10` | Frames of the call stack of a message; `0` keeps only the call site |
| `max_depth`, `max_items`, `max_string_bytes` | `6`, `100`, `2000` | Limits for logged values |
| `mask_keys` | `[]` | More keys to write as `***` |
| `log_globals` | `["get", "post", "files"]` | Superglobals logged at start; add `server`, `session`, `cookie` when needed — they hold secrets |
| `max_age_days`, `max_files`, `max_logs_size` | `30`, `5000`, `268435456` | Old logs are deleted past these limits (bytes for the size); `0` turns a limit off |
| `http_token` | none | Turns the HTTP mode on (at least 32 characters) |
| `http_allowed_ips` | `[]` | Addresses or CIDR networks allowed to use the HTTP mode; empty means any |
| `http_rate_limit` | `600` | HTTP mode requests per minute |

## HTTP mode

For servers the viewer cannot reach over SSH. Off by default; with `http_token` set, the library answers under `/__wapp_debugger/v1/` (status, turning logging on and off, listing, reading and deleting logs — see [`spec/library-http.yaml`](../spec/library-http.yaml)). The token goes only in the `X-Debugger-Token` header; without it the request reaches the application as if the mode did not exist. Use HTTPS.

## Upgrading from library v1

- Replace `require_once 'Debugger.class.php'` with the autoloader (Composer or `autoload.php`) and `Debugger::boot()`.
- `__d`, `__log`, `__start` and `__stop` stay as they are. `fnDebugLog()`, `fnDebugAddFoundField()` and `fnDebugAddNotFoundField()` still work but are deprecated (one `E_USER_DEPRECATED` per request) and go away in 3.0, as do the `DWS_*` constants.
- The old keys of `dws_debugger.json` are still read and the log gets a warning; rename them: `bIsOnFlag` → `enabled`, `sProjectRootPath` → `project_root`, `sMessagesFilesPath` → `logs_dir`, `iArraySizeLimit` → `max_items`, `iLogsSizeLimit` (kilobytes) → `max_logs_size` (bytes), `bLogSessionAndCookies: true` → `log_globals` with `session` and `cookie`; `sTempPath` and `iPageSize` are not used.
- `$_SERVER` is no longer logged by default (add `server` to `log_globals` if you need it).
- The library writes log format v2; the viewer reads both v1 and v2, so old logs stay readable.
