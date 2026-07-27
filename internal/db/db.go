// Package db wires up the embedded SQLite database. There is no external
// database process to install or configure — the data file lives next to
// the binary.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens (creating if necessary) the SQLite database at path and
// applies the pragmas Longtable expects to run with.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", path)

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// SQLite only supports one writer at a time; a single connection
	// avoids "database is locked" errors under concurrent requests.
	database.SetMaxOpenConns(1)

	if err := migrate(database); err != nil {
		database.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return database, nil
}

// migrate applies schema migrations. The schema itself (campaigns, maps,
// tokens, ...) isn't designed yet — this just proves the connection works.
func migrate(database *sql.DB) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS schema_meta (
			version INTEGER NOT NULL
		);
	`)
	return err
}
