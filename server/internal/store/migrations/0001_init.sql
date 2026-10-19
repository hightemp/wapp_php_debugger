-- Схема просмотрщика (п. 7.5 плана): серверы → проекты → запросы → сообщения.
-- Время — INTEGER, миллисекунды от эпохи. Логические значения — INTEGER 0/1. JSON — TEXT с проверкой.

CREATE TABLE servers (
    id            INTEGER PRIMARY KEY,
    name          TEXT    NOT NULL UNIQUE,
    kind          TEXT    NOT NULL CHECK (kind IN ('local', 'ssh', 'http')),
    environment   TEXT    NOT NULL DEFAULT 'local' CHECK (environment IN ('local', 'dev', 'stage', 'prod')),
    -- Параметры подключения без секретов: ssh — host/alias, user, port, jump; http — base_url.
    -- Пароли и токены хранятся в системном хранилище секретов (п. 10.19), не здесь.
    config        TEXT    NOT NULL DEFAULT '{}' CHECK (json_valid(config) AND json_type(config) = 'object'),
    read_only     INTEGER NOT NULL DEFAULT 0 CHECK (read_only IN (0, 1)),
    allow_delete  INTEGER NOT NULL DEFAULT 1 CHECK (allow_delete IN (0, 1)),
    allow_control INTEGER NOT NULL DEFAULT 1 CHECK (allow_control IN (0, 1)),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
) STRICT;

-- Проект принадлежит ровно одному серверу (п. 10.20).
CREATE TABLE projects (
    id                  INTEGER PRIMARY KEY,
    server_id           INTEGER NOT NULL REFERENCES servers (id) ON DELETE CASCADE,
    name                TEXT    NOT NULL,
    root_path           TEXT    NOT NULL,
    logs_dir            TEXT    NOT NULL,
    -- Локальная копия проекта для ссылок в IDE; NULL — ссылки через удалённый путь
    local_path          TEXT,
    editor              TEXT    NOT NULL DEFAULT 'vscode',
    retention_days      INTEGER CHECK (retention_days > 0),
    retention_max_bytes INTEGER CHECK (retention_max_bytes > 0),
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL,
    UNIQUE (server_id, name),
    UNIQUE (server_id, logs_dir)
) STRICT;

-- Состояние чтения файлов логов: позволяет дочитывать растущие файлы с места остановки (локально и по SFTP).
CREATE TABLE log_files (
    id             INTEGER PRIMARY KEY,
    project_id     INTEGER NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    path           TEXT    NOT NULL, -- относительно logs_dir проекта
    format_version INTEGER NOT NULL CHECK (format_version IN (1, 2)),
    size           INTEGER NOT NULL CHECK (size >= 0),
    mtime          INTEGER NOT NULL,
    read_offset    INTEGER NOT NULL DEFAULT 0 CHECK (read_offset >= 0 AND read_offset <= size),
    status         TEXT    NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'complete', 'broken')),
    error          TEXT,
    UNIQUE (project_id, path)
) STRICT;

CREATE TABLE requests (
    id                 INTEGER PRIMARY KEY,
    project_id         INTEGER NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    -- NULL для запросов, пришедших не из файла (приём по HTTP, фаза 10)
    log_file_id        INTEGER REFERENCES log_files (id) ON DELETE CASCADE,
    -- v2: request_id (ULID); v1: iRequestTimestamp строкой
    request_key        TEXT    NOT NULL,
    parent_request_key TEXT,
    sapi               TEXT    NOT NULL CHECK (sapi IN ('web', 'cli')),
    lang               TEXT    NOT NULL DEFAULT 'php' CHECK (lang IN ('php', 'js')),
    method             TEXT,
    url                TEXT,
    is_ajax            INTEGER NOT NULL DEFAULT 0 CHECK (is_ajax IN (0, 1)),
    argv               TEXT    CHECK (argv IS NULL OR (json_valid(argv) AND json_type(argv) = 'array')),
    ip                 TEXT,
    server_ip          TEXT,
    server_name        TEXT,
    hostname           TEXT,
    pid                INTEGER,
    process_user       TEXT,
    app_user           TEXT,
    session_id         TEXT,
    library_version    TEXT,
    php_version        TEXT,
    started_at         INTEGER NOT NULL, -- часы сервера проекта
    ended_at           INTEGER,
    duration_ms        REAL,
    status_code        INTEGER,
    exit_code          INTEGER,
    memory_peak_bytes  INTEGER,
    finished           INTEGER NOT NULL DEFAULT 0 CHECK (finished IN (0, 1)),
    message_count      INTEGER NOT NULL DEFAULT 0,
    error_count        INTEGER NOT NULL DEFAULT 0,
    warning_count      INTEGER NOT NULL DEFAULT 0,
    received_at        INTEGER NOT NULL, -- часы просмотрщика: для оценки сдвига часов серверов
    UNIQUE (project_id, request_key)
) STRICT;

CREATE INDEX requests_project_started ON requests (project_id, started_at DESC);
CREATE INDEX requests_project_parent ON requests (project_id, parent_request_key);
CREATE INDEX requests_log_file ON requests (log_file_id);

-- События запроса, кроме request_start/request_end (они хранятся в requests).
CREATE TABLE messages (
    id              INTEGER PRIMARY KEY,
    request_id      INTEGER NOT NULL REFERENCES requests (id) ON DELETE CASCADE,
    seq             INTEGER NOT NULL CHECK (seq >= 1),
    ts              INTEGER NOT NULL,
    type            TEXT    NOT NULL CHECK (type IN ('message', 'timer', 'error')),
    level           TEXT    NOT NULL CHECK (level IN ('error', 'warning', 'info', 'message', 'debug')),
    message         TEXT    NOT NULL, -- для таймера — его имя
    tags            TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(tags) AND json_type(tags) = 'array'),
    context         TEXT    CHECK (context IS NULL OR (json_valid(context) AND json_type(context) = 'object')),
    source_file     TEXT,
    source_line     INTEGER,
    source_function TEXT,
    source_class    TEXT,
    trace           TEXT    CHECK (trace IS NULL OR (json_valid(trace) AND json_type(trace) = 'array')),
    duration_ms     REAL,    -- таймер
    error_kind      TEXT    CHECK (error_kind IN ('php_error', 'exception', 'fatal')),
    error_type      TEXT,
    exception_class TEXT,
    code            TEXT,
    previous        TEXT    CHECK (previous IS NULL OR (json_valid(previous) AND json_type(previous) = 'array')),
    size_bytes      INTEGER NOT NULL DEFAULT 0,
    UNIQUE (request_id, seq)
) STRICT;

CREATE INDEX messages_request_level ON messages (request_id, level);

-- Полнотекстовый поиск по тексту и контексту. unicode61 приводит регистр, в том числе кириллицу.
CREATE VIRTUAL TABLE messages_fts USING fts5 (
    message,
    context,
    content = 'messages',
    content_rowid = 'id',
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER messages_fts_insert AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts (rowid, message, context) VALUES (new.id, new.message, new.context);
END;

CREATE TRIGGER messages_fts_delete AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts (messages_fts, rowid, message, context) VALUES ('delete', old.id, old.message, old.context);
END;

CREATE TRIGGER messages_fts_update AFTER UPDATE OF message, context ON messages BEGIN
    INSERT INTO messages_fts (messages_fts, rowid, message, context) VALUES ('delete', old.id, old.message, old.context);
    INSERT INTO messages_fts (rowid, message, context) VALUES (new.id, new.message, new.context);
END;

-- Настройки просмотрщика и UI (тема, плотность, редактор — п. 10.25).
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL CHECK (json_valid(value))
) STRICT;
