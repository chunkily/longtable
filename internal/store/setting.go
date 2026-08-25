package store

import (
	"database/sql"
	"errors"
	"time"
)

// Settings is every value a Host has actually set, keyed the way
// internal/config's registry keys them.
//
// Anything absent is absent on purpose rather than empty: the registry
// supplies defaults, so storing one would only make a later change to
// that default invisible to the servers that had already started.
func (s *Store) Settings() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM setting`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, rows.Err()
}

// GetSetting returns one stored value, and whether it was there at all.
//
// The distinction matters to the CLI: "unset, so it's using the default"
// and "set to this" are different answers, and collapsing them would
// leave a Host unable to tell whether they had changed something.
func (s *Store) GetSetting(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM setting WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}

// SetSetting stores a value, replacing whatever was there.
//
// An upsert rather than a check-then-write, because the server and a CLI
// invocation can hold this file open at the same moment — the same
// reason FindOrCreateAsset is one statement. Nothing here validates:
// internal/config's registry does that, and it has to happen before this
// is reached so a bad value never lands in the database at all.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO setting (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

// UnsetSetting removes a stored value, putting the setting back to the
// registry's default. Removing one that was never there is not an error
// — the caller asked for a state, and it is already the state.
func (s *Store) UnsetSetting(key string) error {
	_, err := s.db.Exec(`DELETE FROM setting WHERE key = ?`, key)
	return err
}
