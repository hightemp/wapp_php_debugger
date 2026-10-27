# Форматы логов

[English](README.md) · **Русский**

| Версия | Кто пишет | Схема | Фикстуры |
|---|---|---|---|
| v1 | `client-php/v1/Debugger.class.php` (библиотека 1.x) | [`v1/header.schema.json`](v1/header.schema.json), [`v1/message.schema.json`](v1/message.schema.json) | [`../fixtures/v1`](../fixtures/v1) |
| v2 | `client-php` 2.x | [`v2/event.schema.json`](v2/event.schema.json) | [`../fixtures/v2`](../fixtures/v2) |

Просмотрщик всегда читает обе версии; новая библиотека пишет только v2. Версия файла определяется по его первой строке: у каждого события v2 есть `"v": 2`.

v2 заморожен с 1.0 ([`../frozen`](../frozen/README.ru.md)): он получает только новые поля и типы событий, а логи любой библиотеки 2.x остаются читаемыми.

Схемы проверяются тестами в `server/internal/spec` (`make server-test`).

## v1

Один файл на запрос, названный по времени начала запроса в миллисекундах. Первая строка — заголовок, дополненный пробелами до 1000 байт и перезаписываемый на месте; дальше по строке на сообщение. Известные дефекты, которые читатель должен пережить:

- заголовок длиннее 1000 байт (библиотека до `legacy-hotfix`) затирает начало первого сообщения, и эта строка не разбирается как JSON;
- процесс, убитый до shutdown, оставляет `bFinished: false` и не пишет последних сообщений;
- оборванная последняя строка;
- `sMessage` бывает любым скаляром, а `oVars` — объект даже для списков.

## v2

Один файл на запрос: `<ts начала>-<request_id>.jsonl`, только дописывание (файл никогда не перезаписывается, поэтому читатель может спокойно продолжать с сохранённого offset, в том числе по SFTP). Каждая строка — самодостаточное событие:

| `type` | Когда | Главные поля |
|---|---|---|
| `request_start` | первое событие | `sapi` (`web`/`cli`), `lang`, `library_version`; для web — `method`, `url`; для cli — `argv`; `parent_request_id` для AJAX и дочерних процессов |
| `message` | `__d()`, PSR-3, Monolog, хелперы SQL и HTTP | `level`, `message`, `tags`, `context`, `duration_ms`, `source`, `trace` |
| `timer` | `__stop()` | `name`, `duration_ms`, `level` |
| `error` | ошибки PHP, исключения, фатальные ошибки | `kind` (`php_error`/`exception`/`fatal`), `level`, `message`, `error_type` или `exception_class`, `previous` |
| `request_end` | shutdown | `duration_ms`, `status_code` (web) или `exit_code` (cli), `memory_peak_bytes` |

Поля каждого события: `v` (2), `type`, `request_id` (ULID), `seq`, `ts` (мс от эпохи, часы сервера).

**Правила потока** (проверяются тестами, а не схемой):

1. Первое событие — `request_start`, и оно одно.
2. У всех событий файла один `request_id`.
3. `seq` начинается с 1 и растёт на 1 без пропусков — пропуск означает потерянную строку.
4. `ts` не убывает.
5. Не больше одного `request_end`, и только последним событием. Нет `request_end` — запрос ещё идёт или процесс убит.

**Родительский запрос.** Веб-запрос берёт `parent_request_id` из заголовка `X-Wapp-Debugger-Parent` (страница передаёт свой `request_id` своим AJAX-вызовам); CLI-процесс берёт его из переменной окружения `WAPP_DEBUGGER_PARENT`, которую библиотека выставляет в текущий `request_id`, поэтому процессы, запущенные из запроса (exec, proc_open, задачи очереди, выполняемые на месте), связываются с ним сами. Значение, которое не является ULID, игнорируется.

**Длительности** (`duration_ms` у `timer`, `request_end` и у `message` об операции со временем) измеряются по монотонным часам и бывают дробными; `ts` — настенные часы. Событие с длительностью пишется, когда операция закончилась: на шкале времени запроса оно занимает `[ts - duration_ms, ts]`.

**SQL- и HTTP-вызовы** — сообщения с тегом и длительностью, поэтому просмотрщик может перечислить их и показать время:

- `tags: ["sql"]` — `message` — текст запроса (с плейсхолдерами), `context.params` — его параметры, `context.rows` — затронутые или возвращённые строки, если известно;
- `tags: ["http"]` — `message` — `METHOD url` исходящего запроса, `context.status` — статус ответа, `context.error` — ошибка.

**Уровни:** `error`, `warning`, `info`, `message`, `debug`. Уровни PSR-3 сопоставляются так: `emergency`, `alert`, `critical`, `error` → `error`; `warning` → `warning`; `notice`, `info` → `info`; `debug` → `debug`.

**Имена** — `snake_case`. Пути в `source` и `trace` — относительно корня проекта. Библиотека маскирует секреты перед записью (`"***"`).

### Соответствие v1 → v2

| v1 | v2 |
|---|---|
| `iRequestTimestamp` заголовка | `ts` у `request_start`; ключ запроса v1 — имя файла (настоящие файлы названы по `iRequestTimestamp`, но уникальность в папке логов гарантирована только у имени) |
| `bIsCli` | `sapi` |
| `sURL`, `bIsAjax`, `aArgv`, `sIP`, `sServerIP`, `sServerName` | `url`, `is_ajax`, `argv`, `ip`, `server_ip`, `server_name` |
| `sUser`, `sSysUser`, `sSession`, `sLang` | `process_user`, `app_user`, `session_id`, `lang` |
| `bFinished`, `iCode`, `iTotalTime` | наличие `request_end`, `status_code`, `duration_ms` |
| `sType` | `level` (`functions_debug` и неизвестные типы → `message`) |
| `sMessage` | `message` (скаляры приводятся к строке) |
| теги `#timediff #name` в тексте | `tags`; сообщение `#timediff #name N мс` становится событием `timer` |
| `oVars` | `context` |
| `oBackTrace` (`file`, `line`, `function`) | `source` |
| `aBackTrace` (`class`, `type`) | `trace` (`class`, `call_type`) |
| `iTimestamp` | `ts` |
