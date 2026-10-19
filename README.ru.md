# wapp_php_debugger

[English version](README.md)

Логгер-отладчик для PHP-проектов: PHP-библиотека пишет события каждого запроса в JSONL-логи, просмотрщик показывает запросы, сообщения, бэктрейсы и переменные в реальном времени.

> **Статус: идёт переписывание.** Просмотрщик переписывается на Go + Svelte (с десктоп-версией на Wails), библиотека — в Composer-пакет для PHP 7.4+. План, фазы и решения — в [IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md), правила разработки — в [CONTRIBUTING.md](CONTRIBUTING.md) (на английском).

## Структура

| Каталог | Что |
|---|---|
| `server/` | Новый просмотрщик на Go: `wapp-debugger serve`, API, обновления в реальном времени, встроенный UI |
| `ui/` | Новый веб-интерфейс на Svelte, английский и русский |
| `server/cmd/wapp-debugger-desktop/` | Десктоп-версия на Wails v3 |
| `client-php/` | Новая PHP-библиотека (в разработке) |
| `spec/` | Форматы логов, контракты API и фикстуры |
| `legacy/` | Текущая библиотека v1 и старый просмотрщик |

## Новый просмотрщик (предварительная версия)

Новый просмотрщик уже читает логи текущей библиотеки v1:

```sh
make build                                        # dist/wapp-debugger со встроенным UI
dist/wapp-debugger serve --logs-dir /var/www/app/tmp/logs
# откройте http://127.0.0.1:8820
```

Он слушает только `127.0.0.1`, в фоне импортирует новые и изменившиеся файлы логов и сам показывает новые запросы на странице. Десктоп: `make desktop-build`, затем `dist/wapp-debugger-desktop --logs-dir …`. Язык интерфейса переключается в шапке.

### Доступ по сети

По умолчанию просмотрщик отвечает только на `127.0.0.1` и только на loopback-имена хоста — это заодно защищает от DNS rebinding. Чтобы открыть его другим машинам, задайте токен; без токена просмотрщик откажется слушать не-loopback адрес:

```sh
WAPP_DEBUGGER_TOKEN=$(openssl rand -hex 32) dist/wapp-debugger serve --addr 0.0.0.0:8820 --logs-dir …
# один раз откройте http://<host>:8820/?token=<token>: токен переедет в HttpOnly-cookie
# API-клиенты передают Authorization: Bearer <token>
```

Переменная окружения лучше `--token`: её не видно в списке процессов. Просмотрщик ставит строгий Content-Security-Policy, отклоняет запись с чужих origin и ограничивает тело запроса 1 МиБ. TLS он не терминирует: через недоверенные сети используйте TLS-прокси или SSH-туннель.

## Текущая версия (legacy)

Пока новая версия не выпущена, рабочая — `legacy/` (тег `legacy-hotfix` — со срочными исправлениями безопасности).

**Библиотека.** Подключить `legacy/Debugger.class.php` в проект через `require_once` и положить рядом `dws_debugger.json`:

```json
{
    "sProjectRootPath": "/var/www/app",
    "sTempPath": "/var/www/app/tmp",
    "sMessagesFilesPath": "/var/www/app/tmp/logs",
    "bIsOnFlag": true
}
```

Логирование:

```php
__d(DWS_MESSAGE, 'Заказ создан', ['order' => $order]);
func(__log($a), __log($b)); // перехват значений прямо в выражении
__start('db'); /* ... */ __stop('db'); // таймер
```

Значения с ключами вроде `password`, `token`, `api_key` маскируются. `$_SESSION` и `$_COOKIE` пишутся только при `"bLogSessionAndCookies": true`. HTTP-команды (`?debug_msg=…`) работают только с константой `DA_HTTP_TOKEN` и заголовком `X-Debugger-Token`.

**Старый просмотрщик:**

```sh
cd legacy
./bin_start_server.sh        # http://127.0.0.1:8820
# или
cp .env.example .env && docker compose up -d
```

![](legacy/screenshots/screenshot_01.png)
