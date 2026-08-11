package store

type FogCell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// RevealCells marks cells as revealed for sceneID (idempotent — already
// revealed cells are left as-is).
func (s *Store) RevealCells(sceneID string, cells []FogCell) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO fog_cell (scene_id, cell_x, cell_y) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range cells {
		if _, err := stmt.Exec(sceneID, c.X, c.Y); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// HideCells un-reveals cells for sceneID, the exact inverse of
// RevealCells and equally idempotent — a cell that was never revealed
// simply deletes nothing. Fog is stored as the set of revealed cells
// rather than a per-cell revealed flag, so hiding is a delete and the
// two operations can't disagree about what an absent row means.
func (s *Store) HideCells(sceneID string, cells []FogCell) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`DELETE FROM fog_cell WHERE scene_id = ? AND cell_x = ? AND cell_y = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range cells {
		if _, err := stmt.Exec(sceneID, c.X, c.Y); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ClearFog re-hides a whole scene at once — the classic dungeon-crawl
// starting point, though it's no longer where a scene starts on its
// own; see the fog materialisation in handleSceneCreate.
func (s *Store) ClearFog(sceneID string) error {
	_, err := s.db.Exec(`DELETE FROM fog_cell WHERE scene_id = ?`, sceneID)
	return err
}

func (s *Store) ListFogCells(sceneID string) ([]FogCell, error) {
	rows, err := s.db.Query(`SELECT cell_x, cell_y FROM fog_cell WHERE scene_id = ?`, sceneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cells []FogCell
	for rows.Next() {
		var c FogCell
		if err := rows.Scan(&c.X, &c.Y); err != nil {
			return nil, err
		}
		cells = append(cells, c)
	}
	return cells, rows.Err()
}
