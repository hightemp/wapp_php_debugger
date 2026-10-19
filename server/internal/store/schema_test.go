package store

import (
	"database/sql"
	"strings"
	"testing"
)

// Схема из п. 7.5 плана: servers → projects → requests → messages, плюс log_files (состояние чтения)
// и settings. Тесты фиксируют ограничения, на которые будет опираться код импорта и API.

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) sql.Result {
	t.Helper()
	result, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return result
}

func mustFail(t *testing.T, db *sql.DB, why, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err == nil {
		t.Fatalf("%s: query succeeded: %s", why, query)
	}
}

func lastID(t *testing.T, result sql.Result) int64 {
	t.Helper()
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type fixture struct {
	serverID, projectID, logFileID, requestID int64
}

func seed(t *testing.T, db *sql.DB) fixture {
	t.Helper()
	var f fixture
	f.serverID = lastID(t, mustExec(t, db, `INSERT INTO servers(name, kind, environment, created_at, updated_at) VALUES ('local', 'local', 'local', 1, 1)`))
	f.projectID = lastID(t, mustExec(t, db, `INSERT INTO projects(server_id, name, root_path, logs_dir, created_at, updated_at) VALUES (?, 'shop', '/var/www/shop', '/var/www/shop/tmp/logs', 1, 1)`, f.serverID))
	f.logFileID = lastID(t, mustExec(t, db, `INSERT INTO log_files(project_id, path, format_version, size, mtime, read_offset) VALUES (?, '1792400000000-01JAZ7Q5X0K4M8T2V6N9P3R5S7.jsonl', 2, 100, 1, 0)`, f.projectID))
	f.requestID = lastID(t, mustExec(t, db, `INSERT INTO requests(project_id, log_file_id, request_key, sapi, started_at, received_at) VALUES (?, ?, '01JAZ7Q5X0K4M8T2V6N9P3R5S7', 'web', 1792400000000, 1792400000500)`, f.projectID, f.logFileID))
	mustExec(t, db, `INSERT INTO messages(request_id, seq, ts, type, level, message, context) VALUES (?, 2, 1792400000005, 'message', 'warning', 'Медленный ответ кеша', '{"cache":"redis","ms":120}')`, f.requestID)
	return f
}

func TestSchemaTablesExist(t *testing.T) {
	db := openTestDB(t)
	for _, table := range []string{"servers", "projects", "log_files", "requests", "messages", "messages_fts", "settings"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = ?`, table).Scan(&n); err != nil || n != 1 {
			t.Errorf("table %s: exists=%d err=%v", table, n, err)
		}
	}
}

func TestDeletingServerCascadesToEverything(t *testing.T) {
	db := openTestDB(t)
	f := seed(t, db)
	mustExec(t, db, `DELETE FROM servers WHERE id = ?`, f.serverID)
	for _, table := range []string{"projects", "log_files", "requests", "messages"} {
		if n := count(t, db, table); n != 0 {
			t.Errorf("%s: %d rows left after deleting the server", table, n)
		}
	}
	// Для FTS с внешним содержимым count(*) читает таблицу messages, поэтому проверяем сам индекс
	var stale int
	if err := db.QueryRow(`SELECT count(*) FROM messages_fts WHERE messages_fts MATCH 'кеша'`).Scan(&stale); err != nil {
		t.Fatal(err)
	}
	if stale != 0 {
		t.Fatalf("search index keeps %d entries of deleted messages", stale)
	}
}

func TestDeletingLogFileKeepsNothingOfItsRequest(t *testing.T) {
	db := openTestDB(t)
	f := seed(t, db)
	mustExec(t, db, `DELETE FROM log_files WHERE id = ?`, f.logFileID)
	if n := count(t, db, "requests") + count(t, db, "messages"); n != 0 {
		t.Fatalf("%d rows left: request data must go away together with its log file", n)
	}
}

func TestReferencesAreEnforced(t *testing.T) {
	db := openTestDB(t)
	mustFail(t, db, "project needs a server",
		`INSERT INTO projects(server_id, name, root_path, logs_dir, created_at, updated_at) VALUES (999, 'x', '/x', '/x/logs', 1, 1)`)
	mustFail(t, db, "message needs a request",
		`INSERT INTO messages(request_id, seq, ts, type, level, message) VALUES (999, 1, 1, 'message', 'info', 'x')`)
}

func TestUniqueness(t *testing.T) {
	db := openTestDB(t)
	f := seed(t, db)

	mustFail(t, db, "server names are unique",
		`INSERT INTO servers(name, kind, environment, created_at, updated_at) VALUES ('local', 'ssh', 'dev', 1, 1)`)
	mustFail(t, db, "project names are unique per server",
		`INSERT INTO projects(server_id, name, root_path, logs_dir, created_at, updated_at) VALUES (?, 'shop', '/other', '/other/logs', 1, 1)`, f.serverID)
	mustFail(t, db, "one logs dir belongs to one project",
		`INSERT INTO projects(server_id, name, root_path, logs_dir, created_at, updated_at) VALUES (?, 'shop2', '/var/www/shop', '/var/www/shop/tmp/logs', 1, 1)`, f.serverID)
	mustFail(t, db, "request keys are unique per project",
		`INSERT INTO requests(project_id, request_key, sapi, started_at, received_at) VALUES (?, '01JAZ7Q5X0K4M8T2V6N9P3R5S7', 'web', 1, 1)`, f.projectID)
	mustFail(t, db, "seq is unique per request",
		`INSERT INTO messages(request_id, seq, ts, type, level, message) VALUES (?, 2, 1, 'message', 'info', 'dup')`, f.requestID)

	other := lastID(t, mustExec(t, db, `INSERT INTO servers(name, kind, environment, created_at, updated_at) VALUES ('stage', 'ssh', 'stage', 1, 1)`))
	mustExec(t, db, `INSERT INTO projects(server_id, name, root_path, logs_dir, created_at, updated_at) VALUES (?, 'shop', '/var/www/shop', '/var/www/shop/tmp/logs', 1, 1)`, other)
}

func TestCheckConstraints(t *testing.T) {
	db := openTestDB(t)
	f := seed(t, db)
	cases := []struct{ why, query string }{
		{"unknown server kind", `INSERT INTO servers(name, kind, environment, created_at, updated_at) VALUES ('a', 'ftp', 'dev', 1, 1)`},
		{"unknown environment", `INSERT INTO servers(name, kind, environment, created_at, updated_at) VALUES ('b', 'ssh', 'qa', 1, 1)`},
		{"server config must be JSON", `INSERT INTO servers(name, kind, environment, config, created_at, updated_at) VALUES ('c', 'ssh', 'dev', 'host=x', 1, 1)`},
		{"read_only is boolean", `INSERT INTO servers(name, kind, environment, read_only, created_at, updated_at) VALUES ('d', 'ssh', 'dev', 2, 1, 1)`},
		{"unknown format version", `INSERT INTO log_files(project_id, path, format_version, size, mtime, read_offset) VALUES (1, 'x', 3, 0, 0, 0)`},
		{"offset beyond size", `INSERT INTO log_files(project_id, path, format_version, size, mtime, read_offset) VALUES (1, 'y', 1, 10, 0, 11)`},
		{"unknown sapi", `INSERT INTO requests(project_id, request_key, sapi, started_at, received_at) VALUES (1, 'k', 'fpm', 1, 1)`},
		{"unknown message type", `INSERT INTO messages(request_id, seq, ts, type, level, message) VALUES (1, 10, 1, 'request_start', 'info', 'x')`},
		{"unknown level", `INSERT INTO messages(request_id, seq, ts, type, level, message) VALUES (1, 11, 1, 'message', 'critical', 'x')`},
		{"context must be a JSON object", `INSERT INTO messages(request_id, seq, ts, type, level, message, context) VALUES (1, 12, 1, 'message', 'info', 'x', '[1]')`},
		{"setting value must be JSON", `INSERT INTO settings(key, value) VALUES ('theme', 'dark')`},
	}
	_ = f
	for _, c := range cases {
		t.Run(c.why, func(t *testing.T) { mustFail(t, db, c.why, c.query) })
	}
}

func searchMessages(t *testing.T, db *sql.DB, query string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT m.message FROM messages_fts JOIN messages m ON m.id = messages_fts.rowid WHERE messages_fts MATCH ? ORDER BY m.id`, query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var found []string
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			t.Fatal(err)
		}
		found = append(found, message)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return found
}

func TestFullTextSearchOverMessageAndContext(t *testing.T) {
	db := openTestDB(t)
	f := seed(t, db)
	mustExec(t, db, `INSERT INTO messages(request_id, seq, ts, type, level, message, context) VALUES (?, 3, 1, 'message', 'debug', 'SELECT * FROM items', '{"table":"items"}')`, f.requestID)

	cases := []struct{ query, want string }{
		{"кеша", "Медленный ответ кеша"},
		{"КЕША", "Медленный ответ кеша"},
		{"медлен*", "Медленный ответ кеша"},
		{"redis", "Медленный ответ кеша"},
		{"items", "SELECT * FROM items"},
	}
	for _, c := range cases {
		if got := searchMessages(t, db, c.query); strings.Join(got, "|") != c.want {
			t.Errorf("search %q = %v, want [%s]", c.query, got, c.want)
		}
	}

	mustExec(t, db, `DELETE FROM messages WHERE seq = 3`)
	if got := searchMessages(t, db, "items"); len(got) != 0 {
		t.Fatalf("deleted message still found: %v", got)
	}
}
