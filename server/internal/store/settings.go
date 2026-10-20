package store

import (
	"context"
	"database/sql"
	"errors"
)

// Setting returns the JSON value stored under key.
func (s *Store) Setting(ctx context.Context, key string) ([]byte, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return []byte(value), err == nil, err
}

// SetSetting stores a JSON value under key; the database refuses anything but valid JSON.
func (s *Store) SetSetting(ctx context.Context, key string, value []byte) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`,
		key, string(value))
	return err
}
