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
