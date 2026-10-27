# Схема базы данных просмотрщика

[English](README.md) · **Русский**

SQLite (`modernc.org/sqlite`, без CGO). Источник правды — миграции в [`server/internal/store/migrations/`](../../server/internal/store/migrations): они встроены в бинарник и применяются при открытии базы. Ограничения закреплены тестом `server/internal/store/schema_test.go`.

```
servers ─┬─ projects ─┬─ log_files ── requests ── messages ── messages_fts (FTS5)
         │            └──────────────── requests (log_file_id = NULL: HTTP ingest)
settings
```

| Таблица | Хранит | Главные ограничения |
|---|---|---|
| `servers` | Подключения: `local`, `ssh`, `http`; окружение `local`/`dev`/`stage`/`prod`; права `read_only`, `allow_delete`, `allow_control`; `config` — JSON без секретов | уникальное имя |
| `projects` | Проект на одном сервере: корень, папка логов, локальная копия для ссылок в IDE, редактор, ретеншн | имя и папка логов уникальны в пределах сервера |
| `log_files` | Состояние чтения файла: размер, `mtime`, `read_offset`, версия формата, статус | `read_offset ≤ size`; путь уникален в пределах проекта |
| `requests` | Запрос: данные `request_start`/`request_end` (или заголовок v1), счётчики сообщений и ошибок, `received_at` для сдвига часов | `request_key` уникален в пределах проекта |
| `messages` | События `message`, `timer` и `error` | `seq` уникален в пределах запроса; `context` — объект JSON |
| `messages_fts` | Полнотекстовый индекс по `message` и `context` (unicode61, без учёта регистра, с кириллицей) | синхронизируется триггерами, включая каскадные удаления |
| `settings` | Настройки UI и просмотрщика | значение — JSON |

Удаление сервера, проекта или файла лога каскадом удаляет всё, что ниже. Время — миллисекунды от эпохи (`INTEGER`), логические значения — `0`/`1`, все таблицы `STRICT`.
