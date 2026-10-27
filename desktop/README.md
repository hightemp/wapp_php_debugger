# desktop

**English** · [Русский](README.ru.md)

The desktop viewer is built with Wails v3 on top of `server/internal/*` and the same UI (`ui/`). Its entry point lives in `server/cmd/wapp-debugger-desktop` (build tag `desktop`).

```sh
make desktop-deps              # check the C compiler, GTK and WebKitGTK; prints how to install what is missing
make desktop-build             # build for the current OS (requires CGO; runs desktop-deps first)
make desktop-build LINUX_GTK=gtk3   # Linux with GTK3 + WebKit2GTK 4.1 instead of GTK4
make desktop-selftest          # open a window and check REST and the event stream inside Wails
make desktop-dev LOGS_DIR=/path/to/project/tmp/logs
```

REST goes through the Wails asset handler (no network port). The asset handler buffers whole responses, so live events use a Wails stream instead of SSE.

## Around the window

The decisions live in `server/internal/desktop` and are tested without a window; `server/cmd/wapp-debugger-desktop/shell.go` wires them to Wails:

- `Alerter` turns the event stream into notifications and the tray counter; `RunAlerts` feeds it;
- `ParseLink` / `LinkInArgs` accept only `wapp-debugger://open` with numeric ids;
- `Settings` keeps the desktop settings (`GET/PUT /api/v1/desktop/settings`) and hands saved ones to the shell;
- window size and position are kept in the viewer's database.

macOS sends notifications only for an app with a bundle identifier: a bare binary runs without them.

## Packages

```sh
make icons                     # desktop/icons/icon.svg → PNG, ICO, ICNS (and the tray PNG)
make desktop-package           # installers of the current OS into dist/release-desktop
```

`scripts/desktop-package.sh` builds per OS (unsigned, plan 10.28):

- Linux: `.tar.gz`, `.deb` (`packaging/linux/nfpm.yaml` through `wails3 tool package`), AppImage (`wails3 generate appimage`, which fetches linuxdeploy);
- Windows: `.zip` and an NSIS `_setup.exe` (`packaging/windows/installer.nsi`, needs `makensis`); the icon and manifest go into the `.exe` as a `.syso` resource;
- macOS: a universal `.app` (`lipo` of arm64 and amd64, `packaging/darwin/Info.plist`, ad-hoc signed) in a `.dmg` and a `.tar.gz`.

Every package registers `wapp-debugger://` links: `MimeType` in the `.desktop` file, `HKCU\Software\Classes\wapp-debugger` on Windows, `CFBundleURLTypes` on macOS. The Wails CLI is installed from the version in `server/go.mod` into `.tools/bin`.
