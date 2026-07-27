// Package db wires up the embedded SQLite database. There is no external
// database process to install or configure — the data file lives next to
// the binary. Schema and queries live in internal/store; this package
// only owns the connection.
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

	return database, nil
}
