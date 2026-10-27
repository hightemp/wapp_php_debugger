# desktop

[English](README.md) · **Русский**

Десктоп-версия просмотрщика собирается на Wails v3 поверх `server/internal/*` и того же UI (`ui/`). Точка входа — `server/cmd/wapp-debugger-desktop` (тег сборки `desktop`).

```sh
make desktop-deps              # проверить компилятор C, GTK и WebKitGTK; подсказывает, как поставить недостающее
make desktop-build             # сборка под текущую ОС (нужен CGO; сначала запускает desktop-deps)
make desktop-build LINUX_GTK=gtk3   # Linux с GTK3 + WebKit2GTK 4.1 вместо GTK4
make desktop-selftest          # открыть окно и проверить REST и поток событий внутри Wails
make desktop-dev LOGS_DIR=/path/to/project/tmp/logs
```

REST идёт через asset handler Wails (без сетевого порта). Asset handler буферизует ответы целиком, поэтому живые события идут через поток Wails, а не через SSE.

## Вокруг окна

Решения живут в `server/internal/desktop` и тестируются без окна; `server/cmd/wapp-debugger-desktop/shell.go` подключает их к Wails:

- `Alerter` превращает поток событий в уведомления и счётчик в трее; его питает `RunAlerts`;
- `ParseLink` / `LinkInArgs` принимают только `wapp-debugger://open` с числовыми id;
- `Settings` хранит настройки десктопа (`GET/PUT /api/v1/desktop/settings`) и передаёт сохранённые оболочке;
- размер и положение окна хранятся в базе просмотрщика.

macOS отправляет уведомления только от приложения с bundle identifier: голый бинарник работает без них.

## Пакеты

```sh
make icons                     # desktop/icons/icon.svg → PNG, ICO, ICNS (и PNG для трея)
make desktop-package           # установщики текущей ОС в dist/release-desktop
```

`scripts/desktop-package.sh` собирает под каждую ОС (без подписи, план 10.28):

- Linux: `.tar.gz`, `.deb` (`packaging/linux/nfpm.yaml` через `wails3 tool package`), AppImage (`wails3 generate appimage`, скачивает linuxdeploy);
- Windows: `.zip` и NSIS `_setup.exe` (`packaging/windows/installer.nsi`, нужен `makensis`); иконка и манифест попадают в `.exe` ресурсом `.syso`;
- macOS: universal `.app` (`lipo` из arm64 и amd64, `packaging/darwin/Info.plist`, ad-hoc подпись) в `.dmg` и `.tar.gz`.

Каждый пакет регистрирует ссылки `wapp-debugger://`: `MimeType` в файле `.desktop`, `HKCU\Software\Classes\wapp-debugger` в Windows, `CFBundleURLTypes` в macOS. Wails CLI ставится из версии в `server/go.mod` в `.tools/bin`.
