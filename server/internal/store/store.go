// Package store хранит данные просмотрщика в SQLite: схему, миграции и запросы.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // драйвер "sqlite" без CGO (п. 10.6)
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open открывает (или создаёт) базу и применяет миграции.
func Open(ctx context.Context, file string) (*sql.DB, error) {
	db, err := openDB(file)
	if err != nil {
		return nil, err
	}
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrate(ctx, db, sub); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// openDB открывает базу без миграций. Внешние ключи включаются для каждого соединения пула через DSN.
func openDB(file string) (*sql.DB, error) {
	query := url.Values{}
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "busy_timeout(5000)")

	return sql.Open("sqlite", "file:"+file+"?"+query.Encode())
}

type migration struct {
	version int
	name    string
}

// migrate применяет файлы NNNN_описание.sql по возрастанию версии, каждый в своей транзакции.
func migrate(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		return err
	}

	pending, err := listMigrations(fsys)
	if err != nil {
		return err
	}

	for _, m := range pending {
		var applied int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM schema_migrations WHERE version = ?`, m.version).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		body, err := fs.ReadFile(fsys, m.name)
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, db, m, string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
	}
	return nil
}

func listMigrations(fsys fs.FS) ([]migration, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	result := make([]migration, 0, len(names))
	for _, name := range names {
		prefix, _, found := strings.Cut(path.Base(name), "_")
		version, err := strconv.Atoi(prefix)
		if !found || err != nil || version <= 0 {
			return nil, fmt.Errorf("migration %s: name must start with a positive version, e.g. 0001_init.sql", name)
		}
		result = append(result, migration{version: version, name: name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	return result, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, unixepoch('subsec') * 1000)`, m.version); err != nil {
		return err
	}
	return tx.Commit()
}
