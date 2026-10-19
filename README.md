# wapp_php_debugger

[![CI](https://github.com/hightemp/wapp_php_debugger/actions/workflows/ci.yml/badge.svg)](https://github.com/hightemp/wapp_php_debugger/actions/workflows/ci.yml)
[![](https://asdertasd.site/counter/wapp_avto_express_debugger?a=1)](https://asdertasd.site/counter/wapp_avto_express_debugger)

Логгер-отладчик для PHP-проектов: PHP-библиотека пишет события каждого запроса в JSONL-логи, просмотрщик показывает запросы, сообщения, бэктрейсы и переменные.

> **Статус: идёт переписывание.** Просмотрщик переписывается на Go + Svelte (с десктоп-версией на Wails), библиотека — в Composer-пакет для PHP 7.4+. План, фазы и решения — в [IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md), правила разработки — в [CONTRIBUTING.md](CONTRIBUTING.md).

## Структура

| Каталог | Что |
|---|---|
| `client-php/` | Новая PHP-библиотека (в разработке) |
| `server/` | Новый просмотрщик на Go (в разработке) |
| `ui/` | Новый веб-интерфейс на Svelte (в разработке) |
| `desktop/` | Десктоп-версия на Wails (фаза 4) |
| `spec/` | Фикстуры логов и контракты |
| `legacy/` | Текущая рабочая версия: библиотека v1 и старый просмотрщик |

## Текущая версия (legacy)

Пока новая версия не готова, используется `legacy/` (тег `legacy-hotfix` — со срочными исправлениями безопасности).

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

**Просмотрщик:**

```sh
cd legacy
./bin_start_server.sh        # http://127.0.0.1:8820
# или
cp .env.example .env && docker compose up -d
```

![](legacy/screenshots/screenshot_01.png)
