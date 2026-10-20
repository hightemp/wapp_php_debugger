# wapp_php_debugger

[![CI](https://github.com/hightemp/wapp_php_debugger/actions/workflows/ci.yml/badge.svg)](https://github.com/hightemp/wapp_php_debugger/actions/workflows/ci.yml)
[![](https://asdertasd.site/counter/wapp_avto_express_debugger?a=1)](https://asdertasd.site/counter/wapp_avto_express_debugger)

[Русская версия](README.ru.md)

A logger and debugger for PHP projects: a PHP library writes the events of every request into JSONL logs, and a viewer shows requests, messages, backtraces and variables in real time.

> **Status: early.** The viewer is rewritten in Go + Svelte, with a desktop app on Wails. The PHP library 2.x (PHP 7.4+, log format v2) is ready in this repository and not yet on Packagist; the viewer also reads logs of library v1. The plan, phases and decisions are in [IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md) (in Russian); development rules are in [CONTRIBUTING.md](CONTRIBUTING.md).

![wapp-debugger: request feed, a request and a message with its context and backtrace](docs/screenshot.png)

## Install in one line

```sh
# The viewer (Linux, macOS): picks the build for your system, checks it, puts it into /usr/local/bin or ~/.local/bin
curl -fsSL https://github.com/hightemp/wapp_php_debugger/releases/latest/download/install.sh | sh

# The PHP library as one file, in the root of your project
curl -fsSLO https://github.com/hightemp/wapp_php_debugger/releases/latest/download/wapp-debugger.php
echo '{"enabled": true, "logs_dir": "tmp/logs"}' > dws_debugger.json
```

Then add two lines at the start of the request and run the viewer:

```php
require __DIR__ . '/wapp-debugger.php';
WappDebugger\Debugger::boot(__DIR__ . '/dws_debugger.json');
```

```sh
wapp-debugger serve --logs-dir /var/www/app/tmp/logs   # open http://127.0.0.1:8820
```

The one-liners work from the first published release that carries these files (v0.2.0). Windows: take the zip from [Releases](https://github.com/hightemp/wapp_php_debugger/releases). A specific version: `WAPP_DEBUGGER_VERSION=v0.2.0`, another directory: `WAPP_DEBUGGER_INSTALL_DIR=…` before `sh`.

## Quick start

1. **Or build the viewer from source** (Go 1.26+, Node 22+):

   ```sh
   make ui-deps build                              # dist/wapp-debugger with the embedded UI
   ```

2. **Connect the library 2.x to your project** (PHP 7.4+). Without Composer, copy [`client-php/src`](client-php/src) into the project; then, as early as possible in the request:

   ```php
   require __DIR__ . '/lib/wapp-debugger/autoload.php'; // or Composer's vendor/autoload.php
   WappDebugger\Debugger::boot(__DIR__ . '/dws_debugger.json');
   ```

   and put `dws_debugger.json` into the project root:

   ```json
   {
       "enabled": true,
       "logs_dir": "tmp/logs"
   }
   ```

   The logs directory must be writable by PHP. Log from the code:

   ```php
   __d(DWS_INFO, 'Order created', ['order' => $order]);
   $total = __log($price) * $quantity; // logs a value right inside an expression
   __start('db'); /* ... */ __stop('db'); // timer
   ```

   Errors, exceptions and fatal errors are logged by themselves, secrets are masked. PSR-3, Monolog, all the settings and the upgrade from library v1 are in [client-php/README.md](client-php/README.md). Old projects can keep library v1 ([`client-php/v1/Debugger.class.php`](client-php/v1/Debugger.class.php)): the viewer reads both.

3. **Run the viewer**:

   ```sh
   dist/wapp-debugger serve                                   # empty: add projects in the UI
   dist/wapp-debugger serve --logs-dir /var/www/app/tmp/logs  # or start with one local project
   # open http://127.0.0.1:8820
   ```

   `--logs-dir` is optional. Without it the viewer starts with no projects: add them in the UI ("Add project") — on this machine, or on an SSH or HTTP server added under "Servers" (see [below](#projects-on-other-servers)). Projects are kept in the database, so later runs need no flags; running with the same `--logs-dir` again adds no duplicate.

   With `--logs-dir`, the project root is found by `dws_debugger.json` above the logs directory (else it is the parent directory); `--root`, `--project` and `--config` set the root, the name and the config path explicitly. New and changed log files are imported in the background and new requests appear on the page as they happen. The debugger switch of a project flips `enabled` in `dws_debugger.json` (`bIsOnFlag` of a v1 config).

   Other flags: `--addr` (default `127.0.0.1:8820`; another address needs a token), `--token` (better `$WAPP_DEBUGGER_TOKEN`, out of the process list), `--db` (the database, by default in the user config directory; `secrets.json` with tokens of HTTP servers lives next to it), `--scan-interval` (fallback rescan of local directories, 5 s). `wapp-debugger serve --help` lists them all.

The UI speaks English and Russian and has light, dark and high-contrast themes (switches in the header). Search looks for words in messages and their context or for a part of the URL.

## Projects on other servers

Servers are added under **Servers** in the projects panel; a project then goes to its server ("Add project", the server choice). **Check** on a server shows each step: the connection, the logs directory of each project (readable, writable), the library config and the PHP version.

### SSH

The main way: nothing to open on the server, no extra software there.

- The host is a name or an alias from `~/.ssh/config`: `HostName`, `User`, `Port`, `IdentityFile`, `IdentitiesOnly` and `ProxyJump` chains are honoured. Keys come from `IdentityFile` first, then from ssh-agent (`SSH_AUTH_SOCK`). Passwords and keys with a passphrase not loaded into the agent are not supported yet.
- Host keys are checked against `~/.ssh/known_hosts`. A new host shows its fingerprint and key type with the command to compare it on the server (`ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub`); the confirmed key goes to the viewer's own file, `~/.ssh/known_hosts` is never written. A changed key is refused.
- Logs are read over SFTP and polled every 2 seconds; one connection per server with keepalive and reconnection. The debugger switch rewrites `dws_debugger.json` on the server at once (a temporary file, then rename) and keeps its permissions. A **read-only** server allows neither.
- Logs of PHP inside Docker on that server: point the project to the volume directory on the host.
- IDE links of a remote project open the local copy when the project has one ("Local copy for IDE links"), otherwise the file on the server through VS Code (or Cursor) Remote-SSH.

### HTTP mode of the library

For servers without SSH access (shared hosting). Set `http_token` (32+ characters) in the project's `dws_debugger.json` — the library then answers its own API, see [client-php/README.md](client-php/README.md#http-mode) — and add a server of kind **HTTP** with the application address and that token. Use `https://`: over plain `http://` the token and the logs travel unencrypted, and the UI warns about it. The token is kept outside the database in `secrets.json` next to it, readable by the user only, and never shown again. One HTTP server is one application, so it holds one project; the library knows its logs directory.

## Desktop app

```sh
make desktop-build                                 # needs GTK4 + WebKitGTK 6.0 on Linux
dist/wapp-debugger-desktop --logs-dir /var/www/app/tmp/logs
```

The same viewer in a window, without a network port; `--logs-dir` is optional here too.

## Docker

```sh
docker run -d --name wapp-debugger -e WAPP_DEBUGGER_TOKEN=<token> -p 127.0.0.1:8820:8820 \
  --user "$(id -u):$(id -g)" -v /var/www/app:/var/www/app -v wapp-debugger:/data \
  ghcr.io/hightemp/wapp-debugger --logs-dir /var/www/app/tmp/logs
# open http://127.0.0.1:8820/?token=<token>
```

Inside a container the viewer listens on all interfaces, so the token is required. Mount the project at the same path as on the host: paths in the viewer and IDE links then match. `--user` lets the viewer read the logs and switch the debugger in `dws_debugger.json`. `make docker-build` builds the image locally.

## Network access

By default the viewer answers only on `127.0.0.1` and only to loopback host names, which also blocks DNS rebinding. To open it to other machines, set a token; without one the viewer refuses to listen on a non-loopback address:

```sh
WAPP_DEBUGGER_TOKEN=$(openssl rand -hex 32) dist/wapp-debugger serve --addr 0.0.0.0:8820 --logs-dir …
# open http://<host>:8820/?token=<token> once: the token moves to an HttpOnly cookie
# API clients send Authorization: Bearer <token>
```

Prefer the environment variable over `--token`: it does not show up in the process list. The viewer sets a strict Content-Security-Policy, rejects cross-origin writes and limits request bodies to 1 MiB. It does not terminate TLS: across untrusted networks put it behind a TLS proxy or an SSH tunnel.

The library itself has no HTTP commands by default (`?debug_msg=…` needs the `DA_HTTP_TOKEN` constant and the `X-Debugger-Token` header); the new viewer does not use them.

## Layout

| Directory | What |
|---|---|
| `server/` | Viewer in Go: `wapp-debugger serve`, API, live updates, embedded UI |
| `ui/` | Web UI in Svelte, English and Russian |
| `server/cmd/wapp-debugger-desktop/` | Desktop app on Wails v3 |
| `client-php/` | PHP library 2.x; `v1/` keeps library v1 for old projects |
| `spec/` | Log formats, API contracts and fixtures |

The old PHP viewer with EasyUI is gone; it stays in the git history (tag `legacy-hotfix` is its last version).

## License

[MIT](LICENSE)
