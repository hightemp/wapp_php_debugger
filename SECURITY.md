# Security

**English** · [Русский](SECURITY.ru.md)

## Supported versions

Fixes go into the latest release of the viewer and the desktop app (1.x) and into the latest 2.x
of the PHP library. The legacy viewer and library v1 (`legacy/`, `client-php/v1`) get no fixes.

## Reporting a problem

Please report a vulnerability privately: **Security → Report a vulnerability** on GitHub
(https://github.com/hightemp/wapp_php_debugger/security/advisories/new), not in an issue. Say what
part it is in (viewer, desktop app, library, agent), the version, and how to reproduce it. An answer
comes within a week; a fix is released as soon as it is ready, and the advisory names you unless you
ask otherwise.

## What the project defends

- The viewer listens on 127.0.0.1 by default; another address needs a token, and writes check the
  Origin (CSRF) and the Host (DNS rebinding).
- The receiver of pushed logs (`--ingest-addr`) takes only logs, with a token per project, kept as
  a SHA-256 hash.
- The library is off without its config, masks secrets by key, and its HTTP mode is off without a
  token of 32 characters or more.
- Releases have checksums, SBOMs and build provenance attestations
  (`gh attestation verify <file> --repo hightemp/wapp_php_debugger`).

Logs hold application data by nature: whoever reads the viewer reads them.
