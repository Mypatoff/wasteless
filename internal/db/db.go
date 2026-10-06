// Package db opens the SQLite database, tunes it for a single-process
// local app, and applies schema migrations.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the "sqlite" driver used by database/sql
)

// schemaV1 creates the entries table and its date index.
const schemaV1 = `
CREATE TABLE entries (
	id INTEGER PRIMARY KEY,
	kind TEXT NOT NULL CHECK(kind IN ('saving','spend')),
	amount INTEGER NOT NULL CHECK(amount > 0),
	category TEXT NOT NULL,
	wasted INTEGER NOT NULL DEFAULT 0 CHECK(wasted IN (0,1)),
	note TEXT NOT NULL DEFAULT '',
	date TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	CHECK(kind = 'spend' OR wasted = 0)
);
`

const schemaV1Index = `CREATE INDEX idx_entries_date ON entries(date);`

// Open opens (creating if needed) the SQLite file at path, applies the
// pragmas this app relies on, and migrates the schema.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	// A single connection keeps this simple: no need to worry about
	// SQLite's limited concurrent-writer story across pooled conns.
	database.SetMaxOpenConns(1)

	for _, pragma := range []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := database.Exec(pragma); err != nil {
			return nil, fmt.Errorf("set pragma %q: %w", pragma, err)
		}
	}

	if err := migrate(database); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return database, nil
}

// migrate applies schema changes in order, tracked by PRAGMA user_version,
// so re-running Open on an existing database is always safe.
func migrate(database *sql.DB) error {
	var version int
	if err := database.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	if version < 1 {
		for _, stmt := range []string{schemaV1, schemaV1Index} {
			if _, err := database.Exec(stmt); err != nil {
				return fmt.Errorf("apply v1 schema: %w", err)
			}
		}
		if _, err := database.Exec(`PRAGMA user_version = 1`); err != nil {
			return fmt.Errorf("set user_version to 1: %w", err)
		}
	}

	return nil
}
