# Форматы логов

| Версия | Кто пишет | Схема | Фикстуры |
|---|---|---|---|
| v1 | `legacy/Debugger.class.php` (библиотека 1.x) | [`v1/header.schema.json`](v1/header.schema.json), [`v1/message.schema.json`](v1/message.schema.json) | [`../fixtures/v1`](../fixtures/v1) |
| v2 | `client-php` 2.x | [`v2/event.schema.json`](v2/event.schema.json) | [`../fixtures/v2`](../fixtures/v2) |

Просмотрщик читает обе версии всегда (п. 10.12 плана), новая библиотека пишет только v2. Версия файла определяется по первой строке: в v2 у каждого события есть `"v": 2`.

Схемы проверяются тестами `server/internal/spec` (`make server-test`).

## v1

Один файл на запрос, имя — время начала запроса в мс. Первая строка — заголовок, дополненный пробелами до 1000 байт и перезаписываемый на месте; дальше по строке на сообщение. Известные дефекты, которые читатель обязан переживать:

- заголовок длиннее 1000 байт (библиотека до `legacy-hotfix`) затирает начало первого сообщения — эта строка не разбирается как JSON;
- процесс убит до shutdown — `bFinished: false`, итоговых сообщений нет;
- оборванная последняя строка;
- `sMessage` бывает любым скаляром, `oVars` — объект даже для списков.

## v2

Один файл на запрос: `<ts начала>-<request_id>.jsonl`, только дописывается в конец (никакой перезаписи — файл можно безопасно дочитывать с сохранённого смещения, в том числе по SFTP). Каждая строка — самодостаточное событие:

| `type` | Когда | Главные поля |
|---|---|---|
| `request_start` | первое событие | `sapi` (`web`/`cli`), `lang`, `library_version`; для web — `method`, `url`; для cli — `argv`; `parent_request_id` для AJAX и дочерних процессов |
| `message` | `__d()`, PSR-3, Monolog | `level`, `message`, `tags`, `context`, `source`, `trace` |
| `timer` | `__stop()` | `name`, `duration_ms`, `level` |
| `error` | ошибки PHP, исключения, фатальные ошибки | `kind` (`php_error`/`exception`/`fatal`), `level`, `message`, `error_type` или `exception_class`, `previous` |
| `request_end` | shutdown | `duration_ms`, `status_code` (web) или `exit_code` (cli), `memory_peak_bytes` |

Общие поля каждого события: `v` (2), `type`, `request_id` (ULID), `seq`, `ts` (мс от эпохи, часы сервера).

**Правила потока** (проверяются тестом, не схемой):

1. Первое событие — `request_start`, и оно единственное.
2. У всех событий файла один `request_id`.
3. `seq` начинается с 1 и растёт на 1 без пропусков — пропуск означает потерянную строку.
4. `ts` не убывает.
5. `request_end` — не больше одного и только последним. Его отсутствие означает, что запрос ещё выполняется или процесс был убит.

**Уровни:** `error`, `warning`, `info`, `message`, `debug`. Уровни PSR-3: `emergency`, `alert`, `critical`, `error` → `error`; `warning` → `warning`; `notice`, `info` → `info`; `debug` → `debug`.

**Имена** — `snake_case` без венгерской нотации (п. 4.4). Пути в `source` и `trace` — относительно корня проекта. Секреты маскируются библиотекой до записи (`"***"`).

### Соответствие v1 → v2

| v1 | v2 |
|---|---|
| заголовок `iRequestTimestamp` | `ts` у `request_start`; `request_id` для v1 = строка `iRequestTimestamp` |
| `bIsCli` | `sapi` |
| `sURL`, `bIsAjax`, `aArgv`, `sIP`, `sServerIP`, `sServerName` | `url`, `is_ajax`, `argv`, `ip`, `server_ip`, `server_name` |
| `sUser`, `sSysUser`, `sSession`, `sLang` | `process_user`, `app_user`, `session_id`, `lang` |
| `bFinished`, `iCode`, `iTotalTime` | наличие `request_end`, `status_code`, `duration_ms` |
| `sType` | `level` (`functions_debug` и неизвестные → `message`) |
| `sMessage` | `message` (скаляр приводится к строке) |
| теги `#timediff #name` в тексте | `tags`; сообщение `#timediff #name N мс` → событие `timer` |
| `oVars` | `context` |
| `oBackTrace` (`file`, `line`, `function`) | `source` |
| `aBackTrace` (`class`, `type`) | `trace` (`class`, `call_type`) |
| `iTimestamp` | `ts` |
