package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// openEmptyDB — база без миграций, для тестов самого механизма миграций.
func openEmptyDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := openDB(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func appliedVersions(t *testing.T, db *sql.DB) []int {
	t.Helper()
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, version)
	}
	return versions
}

func TestOpenEnablesForeignKeysAndWAL(t *testing.T) {
	db := openTestDB(t)
	var foreignKeys int
	var journalMode string
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || journalMode != "wal" {
		t.Fatalf("foreign_keys=%d journal_mode=%s, want 1 and wal", foreignKeys, journalMode)
	}
}

func TestMigrateAppliesFilesInVersionOrderOnce(t *testing.T) {
	db := openEmptyDB(t)
	fsys := fstest.MapFS{
		"0010_second.sql": {Data: []byte(`INSERT INTO log(step) VALUES ('second');`)},
		"0002_first.sql":  {Data: []byte(`CREATE TABLE log(step TEXT); INSERT INTO log(step) VALUES ('first');`)},
		"README.md":       {Data: []byte(`not a migration`)},
	}
	for range 2 {
		if err := migrate(context.Background(), db, fsys); err != nil {
			t.Fatal(err)
		}
	}

	var steps string
	if err := db.QueryRow(`SELECT group_concat(step, ',') FROM log`).Scan(&steps); err != nil {
		t.Fatal(err)
	}
	if steps != "first,second" {
		t.Fatalf("steps = %q, want each migration once in version order", steps)
	}
	if got := appliedVersions(t, db); len(got) != 2 || got[0] != 2 || got[1] != 10 {
		t.Fatalf("applied versions = %v", got)
	}
}

func TestMigrateRollsBackFailedMigration(t *testing.T) {
	db := openEmptyDB(t)
	fsys := fstest.MapFS{
		"0001_ok.sql":     {Data: []byte(`CREATE TABLE ok(id INTEGER);`)},
		"0002_broken.sql": {Data: []byte(`CREATE TABLE half(id INTEGER); SELECT * FROM missing_table;`)},
	}
	if err := migrate(context.Background(), db, fsys); err == nil {
		t.Fatal("broken migration must fail")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'half'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("changes of the failed migration must be rolled back")
	}
	if got := appliedVersions(t, db); len(got) != 1 || got[0] != 1 {
		t.Fatalf("applied versions = %v, want [1]", got)
	}
}

func TestMigrateRejectsFileWithoutVersion(t *testing.T) {
	db := openEmptyDB(t)
	err := migrate(context.Background(), db, fstest.MapFS{"init.sql": {Data: []byte(`SELECT 1;`)}})
	if err == nil {
		t.Fatal("migration file without numeric prefix must be rejected")
	}
}
