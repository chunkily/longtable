package store

import "time"

// RecordAssetSource notes that the file hashed as sourceHash was turned
// into this asset. Recording the same pair again — a file uploaded twice
// — keeps the first sighting rather than failing, since the second upload
// is the ordinary way people find out a picture was already there.
//
// Nothing reads these yet. They're kept because they can't be recovered
// later: the original bytes are thrown away at re-encode, so an upload
// made before this existed can only ever be blocked by its output hash.
func (s *Store) RecordAssetSource(assetID, sourceHash string) error {
	_, err := s.db.Exec(
		`INSERT INTO asset_source (asset_id, source_hash, first_seen_at) VALUES (?, ?, ?)
		 ON CONFLICT (asset_id, source_hash) DO NOTHING`,
		assetID, sourceHash, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

// AssetSources lists the source hashes recorded against an asset.
func (s *Store) AssetSources(assetID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT source_hash FROM asset_source WHERE asset_id = ? ORDER BY first_seen_at, rowid`, assetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hashes := []string{}
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, rows.Err()
}

// AssetsFromSource lists every asset made from one source file — the
// grouping a Host's asset list needs, so two crops of one portrait show
// up together rather than one being removed while the other stays.
func (s *Store) AssetsFromSource(sourceHash string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT asset_id FROM asset_source WHERE source_hash = ? ORDER BY first_seen_at, rowid`, sourceHash,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
