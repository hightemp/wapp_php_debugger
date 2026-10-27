# Roadmap

**English** · [Русский](ROADMAP.ru.md)

What 1.0 has and what comes after it. The detailed working plan is
[IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md); ideas and bugs go to the issues.

## 1.0

- **Viewer** (`wapp-debugger serve`) and **desktop app** (Linux, macOS, Windows) on one core: projects
  on this machine, over SSH, over the HTTP mode of the library, or pushed to the viewer; many servers
  with their environments.
- **PHP library 2.x** for PHP 7.4+, no dependencies: errors, exceptions, SQL and HTTP calls, timers,
  masking of secrets; PSR-3, Monolog, Guzzle; Laravel, Symfony, WordPress and 1C-Bitrix
  ([integrations](client-php/integrations/README.md)); a JS client for the errors of pages; an agent
  for hosts without SSH.
- **UI**: timeline, SQL with N+1, superglobals, the source around a line, filters, the command
  palette, comparison of requests, export and import, themes of your own.
- **Contracts frozen** ([spec/frozen](spec/frozen/README.md)): log format v2 and API v1 change only
  in a backward compatible way during 1.x.

## Next

**Viewer**

- Panels that resize (the sidebar already folds and the layout narrows; servers group their projects).
- A page of project settings with path mapping (server or container ↔ local copy) and its check.
- A status line: the live stream, each server, files not imported yet, disk space.
- A raw JSON tab of a message; an overview and an errors tab of a request; saved filters and a
  date range. (First steps for an empty viewer and help in the app are done.)

**Sources**

- Live tail over SSH (`inotifywait`/`tail -F` on the server) instead of polling; compression and a
  rate limit per server.
- Retention quotas per server; a config file of servers and projects to edit by hand.
- Key passphrases and SSH passwords in the system keychain.

**Desktop**

- The desktop app open to a browser as well (a local port with a token).
- A title bar in the theme, the layout of panels remembered, logs dropped onto the window.
- Signed and notarized builds (Developer ID, Authenticode).

**Library**

- Packagist (a split of `client-php/` into its own repository).
- Framework packages (a Laravel service provider with discovery, a Symfony bundle) if there is
  demand: the integration files cover the same today.
