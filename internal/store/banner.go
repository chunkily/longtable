package store

// GetBanner returns the Host's current server-wide message, or "" if
// none is set.
//
// Read fresh on every call rather than cached anywhere — see SetBanner
// for why this table exists at all. The row is seeded once in
// createTables, so this can assume it's there rather than handling
// sql.ErrNoRows.
func (s *Store) GetBanner() (string, error) {
	var message string
	err := s.db.QueryRow(`SELECT message FROM banner`).Scan(&message)
	return message, err
}

// SetBanner replaces the Host's server-wide message. "" clears it.
//
// A table rather than a config file or a command-line flag, so
// `longtable set-banner` and `longtable clear-banner` can change it
// while the server keeps running: they open the same SQLite file the
// server has open, the way `longtable room reset-password` already
// does, and every later GET /api/notice reads whatever is there now.
// No signal, no restart, no reload logic to write or get wrong.
func (s *Store) SetBanner(message string) error {
	_, err := s.db.Exec(`UPDATE banner SET message = ?`, message)
	return err
}
