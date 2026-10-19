# wapp-php-debugger: PHP-библиотека 2.x

[English version](README.md)

Отладочная часть [wapp_php_debugger](../README.ru.md): пишет события каждого запроса (сообщения, таймеры, ошибки, итог) в JSONL-логи, по файлу на запрос, а просмотрщик показывает их вживую. PHP 7.4 и новее, обязательных зависимостей нет.

## Установка

Через Composer:

```sh
composer require --dev hightemp/wapp-php-debugger
```

В Packagist пакета пока нет. До публикации укажите Composer на копию этого репозитория:

```sh
composer config repositories.wapp-debugger path /path/to/wapp_php_debugger/client-php
composer require --dev hightemp/wapp-php-debugger:@dev
```

Без Composer возьмите сборку одним файлом из релиза (всё, кроме мостов PSR-3 и Monolog — им Composer нужен в любом случае):

```sh
curl -fsSLO https://github.com/hightemp/wapp_php_debugger/releases/latest/download/wapp-debugger.php
```

```php
require __DIR__ . '/wapp-debugger.php';
```

или скопируйте `client-php/src` в проект и подключите `src/autoload.php`. Из исходников один файл собирает `make php-bundle`.

## Запуск

Вызовите `boot()` как можно раньше — во фронт-контроллере или в точке входа CLI:

```php
WappDebugger\Debugger::boot(__DIR__ . '/dws_debugger.json');
```

и положите `dws_debugger.json` в корень проекта:

```json
{
    "enabled": true,
    "logs_dir": "tmp/logs"
}
```

Без файла или с `"enabled": false` библиотека ничего не делает: запрос стоит один `is_file()` и чтение маленького JSON. Включённая, `boot()` начинает запрос, регистрирует обработчики ошибок (обработчики приложения продолжают работать, PHP по-прежнему печатает ошибки) и при завершении записывает итог с кодом ответа или кодом выхода. `enabled` переключает просмотрщик.

## Логирование

```php
__d(DWS_INFO, 'Заказ создан', ['order' => $order]);   // уровень, сообщение, именованные переменные
$total = __log($price) * $quantity;                    // пишет значение и возвращает его
__start('db'); /* ... */ __stop('db');                 // таймер: info, warning дольше 1 с, error дольше 2 с
```

То же через объект:

```php
use WappDebugger\Debugger;
use WappDebugger\Level;

$debugger = Debugger::instance();
$debugger->log(Level::DEBUG, 'SELECT * FROM items WHERE id = ?', ['params' => [$id]], ['sql']); // теги
$debugger->exception($caught);          // с трейсом и предыдущими исключениями
$debugger->startTimer('render');
$debugger->stopTimer('render');
```

Уровни: `error`, `warning`, `info`, `message` (нейтральный), `debug`. У каждого события есть место вызова (файл и строка относительно корня проекта) и по умолчанию последние 10 кадров стека.

Значения записываются безопасно: глубина, число элементов и длина строк ограничены, объекты показываются со всеми свойствами, рекурсия обрезается. Ключи, похожие на секреты (`password`, `token`, `api_key`, `authorization`, `cookie`, `dsn`… — то же правило, что в библиотеке v1), пишутся как `***`.

### Родительские запросы

Страница и её AJAX-запросы, задание и запущенные им процессы связываются в просмотрщике:

- веб: передавайте id страницы в заголовке `X-Wapp-Debugger-Parent` её AJAX-запросов; страница берёт его из `Debugger::instance()->requestId()`;
- CLI: ничего делать не нужно — библиотека сама выставляет `WAPP_DEBUGGER_PARENT` для процессов, запущенных из запроса.

## PSR-3 и Monolog

```php
$logger = new WappDebugger\Bridge\PsrLogger();                     // Psr\Log\LoggerInterface: psr/log 1, 2 или 3
$monolog->pushHandler(new WappDebugger\Bridge\MonologHandler());   // Monolog 2 или 3 — какой установлен
```

Уровни PSR-3 сводятся к пяти уровням выше (`critical`, `alert`, `emergency` → `error`, `notice` → `info`); исключение в ключе `exception` контекста пишется с трейсом. Laravel: добавьте обработчик в канал в `config/logging.php` (`'driver' => 'monolog', 'handler' => WappDebugger\Bridge\MonologHandler::class`). Symfony: зарегистрируйте его как сервис-обработчик Monolog.

## Настройки

`dws_debugger.json`; относительные пути считаются от его папки. Права файлов и папок — восьмеричные строки: в JSON нет восьмеричных чисел.

| Ключ | По умолчанию | Что значит |
|---|---|---|
| `enabled` | `false` | Писать логи |
| `project_root` | папка файла | Пути в логах — относительно него |
| `logs_dir` | `<project_root>/tmp/logs` | Куда писать логи; PHP должен иметь право записи |
| `file_mode`, `dir_mode` | `"0664"`, `"0775"` | Права новых файлов и папок (просмотрщик часто работает от другого пользователя) |
| `trace_frames` | `10` | Кадры стека у сообщения; `0` — только место вызова |
| `max_depth`, `max_items`, `max_string_bytes` | `6`, `100`, `2000` | Ограничения для записываемых значений |
| `mask_keys` | `[]` | Дополнительные ключи, которые пишутся как `***` |
| `log_globals` | `["get", "post", "files"]` | Суперглобальные массивы при старте; `server`, `session`, `cookie` — только по необходимости, в них секреты |
| `max_age_days`, `max_files`, `max_logs_size` | `30`, `5000`, `268435456` | Старые логи удаляются сверх этих лимитов (размер в байтах); `0` выключает лимит |
| `http_token` | нет | Включает HTTP-режим (не короче 32 символов) |
| `http_allowed_ips` | `[]` | Адреса или CIDR-сети, которым доступен HTTP-режим; пусто — любые |
| `http_rate_limit` | `600` | Запросов HTTP-режима в минуту |

## HTTP-режим

Для серверов, куда просмотрщик не может зайти по SSH. По умолчанию выключен; с `http_token` библиотека отвечает на путях `/__wapp_debugger/v1/` (состояние, включение и выключение, список, чтение и удаление логов — см. [`spec/library-http.yaml`](../spec/library-http.yaml)). Токен передаётся только в заголовке `X-Debugger-Token`; без него запрос уходит в приложение, как будто режима нет. Используйте HTTPS.

## Переход с библиотеки v1

- Замените `require_once 'Debugger.class.php'` на автозагрузчик (Composer или `autoload.php`) и `Debugger::boot()`.
- `__d`, `__log`, `__start` и `__stop` остаются как есть. `fnDebugLog()`, `fnDebugAddFoundField()` и `fnDebugAddNotFoundField()` работают, но устарели (одно `E_USER_DEPRECATED` за запрос) и уйдут в 3.0 вместе с константами `DWS_*`.
- Старые ключи `dws_debugger.json` по-прежнему читаются, а в лог пишется предупреждение; переименуйте их: `bIsOnFlag` → `enabled`, `sProjectRootPath` → `project_root`, `sMessagesFilesPath` → `logs_dir`, `iArraySizeLimit` → `max_items`, `iLogsSizeLimit` (килобайты) → `max_logs_size` (байты), `bLogSessionAndCookies: true` → `log_globals` с `session` и `cookie`; `sTempPath` и `iPageSize` не используются.
- `$_SERVER` по умолчанию больше не пишется (добавьте `server` в `log_globals`, если нужен).
- Библиотека пишет формат логов v2; просмотрщик читает и v1, и v2, так что старые логи остаются доступны.
