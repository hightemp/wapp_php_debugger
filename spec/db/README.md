# Схема БД просмотрщика

SQLite (`modernc.org/sqlite`, без CGO). Источник истины — миграции [`server/internal/store/migrations/`](../../server/internal/store/migrations), они встроены в бинарник и применяются при открытии базы. Ограничения схемы зафиксированы тестами `server/internal/store/schema_test.go`.

```
servers ─┬─ projects ─┬─ log_files ── requests ── messages ── messages_fts (FTS5)
         │            └──────────────── requests (log_file_id = NULL: приём по HTTP)
settings
```

| Таблица | Что хранит | Ключевые ограничения |
|---|---|---|
| `servers` | Подключения: `local`, `ssh`, `http`; окружение `local`/`dev`/`stage`/`prod`; права `read_only`, `allow_delete`, `allow_control`; `config` — JSON без секретов | имя уникально |
| `projects` | Проект на одном сервере (п. 10.20): корень, папка логов, локальная копия для IDE, редактор, ретеншн | имя и папка логов уникальны в пределах сервера |
| `log_files` | Состояние чтения файла: размер, `mtime`, `read_offset`, версия формата, статус | `read_offset ≤ size`; путь уникален в проекте |
| `requests` | Запрос: данные `request_start`/`request_end` (или заголовка v1), счётчики сообщений и ошибок, `received_at` для сдвига часов | `request_key` уникален в проекте |
| `messages` | События `message`, `timer`, `error` | `seq` уникален в запросе; `context` — JSON-объект |
| `messages_fts` | Полнотекстовый индекс по `message` и `context` (unicode61, регистр и кириллица) | синхронизируется триггерами, в том числе при каскадном удалении |
| `settings` | Настройки UI и просмотрщика | значение — JSON |

Удаление сервера, проекта или файла лога каскадно удаляет всё, что под ним. Время — миллисекунды от эпохи (`INTEGER`), логические значения — `0`/`1`, все таблицы `STRICT`.
