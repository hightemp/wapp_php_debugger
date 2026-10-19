# desktop

The desktop viewer is built with Wails v3 on top of `server/internal/*` and the same UI (`ui/`). Its entry point lives in `server/cmd/wapp-debugger-desktop` (build tag `desktop`).

```sh
make desktop-build             # build for the current OS (requires CGO)
make desktop-build LINUX_GTK=gtk3   # Linux with GTK3 + WebKit2GTK 4.1 instead of GTK4
make desktop-selftest          # open a window and check REST and the event stream inside Wails
make desktop-dev LOGS_DIR=/path/to/project/tmp/logs
```

REST goes through the Wails asset handler (no network port). The asset handler buffers whole responses, so live events use a Wails stream instead of SSE.
