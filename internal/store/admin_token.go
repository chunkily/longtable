package store

import (
	"time"

	"longtable/internal/auth"
)

// AdminToken returns the secret that opens the admin page, minting one
// if this server has never had it asked for.
//
// Minted on demand rather than at migration time so the same call works
// on a database made before this table existed, and so a rotation and a
// first start are the same code path. `auth.NewToken` is 32 bytes of
// crypto/rand rendered URL-safe, which is what lets it be handed over as
// part of a URL.
func (s *Store) AdminToken() (string, error) {
	var token string
	if err := s.db.QueryRow(`SELECT token FROM admin_token`).Scan(&token); err != nil {
		return "", err
	}
	if token != "" {
		return token, nil
	}
	return s.RotateAdminToken()
}

// RotateAdminToken issues a new secret and forgets the old one.
//
// The Host's answer to having pasted the URL somewhere they regret, and
// the reason the token is worth storing rather than deriving.
func (s *Store) RotateAdminToken() (string, error) {
	token, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	if _, err := s.db.Exec(
		`UPDATE admin_token SET token = ?, created_at = ?`,
		token, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return "", err
	}
	return token, nil
}
