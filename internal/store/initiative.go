package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// InitiativeEntry is one combatant in a room's turn order.
//
// A room has exactly one tracker — an encounter at a time is what a
// table runs — so entries hang off the room rather than off a scene.
// That is deliberate and survives a scene switch: a GM flipping to the
// battle map mid-fight must not lose the order.
//
// TokenID links the entry to something on the map, and is nil for a
// freestanding entry: lair actions, environmental hazards, a creature
// nobody has drawn yet. A linked entry takes its name and art from the
// token every time it is sent, rather than copying them at creation, so
// renaming a token renames its entry too.
//
// Hidden is the *entry's* own flag and only means anything for a
// freestanding one. A linked entry's visibility is its token's — one
// answer, from the token, rather than two that can disagree.
type InitiativeEntry struct {
	ID         string
	RoomID     string
	TokenID    *string
	Name       string
	Initiative float64
	Hidden     bool
	// SortOrder breaks ties, and only ties: the order is initiative
	// descending first. It is what "move this one above that one" writes
	// to, so it exists for the case a table settles by argument rather
	// than by dexterity.
	SortOrder int
	CreatedAt string
}

// InitiativeState is the whole tracker: the entries, which one is
// taking its turn, and the round number.
//
// Round and CurrentEntryID live on the room rather than in a table of
// their own — there is one tracker, so they are one row's worth of
// state, and a separate table would be a join for two scalars.
type InitiativeState struct {
	Entries        []InitiativeEntry
	Round          int
	CurrentEntryID *string
}

const initiativeColumns = `id, room_id, token_id, name, initiative, hidden, sort_order, created_at`

// initiativeOrder is the sort every read shares: highest initiative
// first, ties broken by the manual order, then by age, and anything
// still tied by rowid so the result is stable rather than whatever
// SQLite feels like.
//
// That last step is the one doing real work here, and age alone was not
// enough: created_at comes from a clock about a millisecond wide, and
// eight goblins added in one go share an initiative *and* a sort_order
// *and* a timestamp — which is the tracker's most ordinary case rather
// than an edge one. Without it the turn order reshuffled itself between
// reads. See ListRecentMessages for the long version.
const initiativeOrder = ` ORDER BY initiative DESC, sort_order ASC, created_at ASC, rowid ASC`

func (s *Store) CreateInitiativeEntry(e InitiativeEntry) (InitiativeEntry, error) {
	e.ID = uuid.NewString()
	e.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	if _, err := s.db.Exec(
		`INSERT INTO initiative_entry (`+initiativeColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.RoomID, e.TokenID, e.Name, e.Initiative, e.Hidden, e.SortOrder, e.CreatedAt,
	); err != nil {
		return InitiativeEntry{}, err
	}
	return e, nil
}

func (s *Store) GetInitiativeEntry(id string) (InitiativeEntry, error) {
	var e InitiativeEntry
	err := s.db.QueryRow(`SELECT `+initiativeColumns+` FROM initiative_entry WHERE id = ?`, id).
		Scan(&e.ID, &e.RoomID, &e.TokenID, &e.Name, &e.Initiative, &e.Hidden, &e.SortOrder, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return InitiativeEntry{}, ErrNotFound
		}
		return InitiativeEntry{}, err
	}
	return e, nil
}

func (s *Store) ListInitiativeEntries(roomID string) ([]InitiativeEntry, error) {
	rows, err := s.db.Query(
		`SELECT `+initiativeColumns+` FROM initiative_entry WHERE room_id = ?`+initiativeOrder, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []InitiativeEntry{}
	for rows.Next() {
		var e InitiativeEntry
		if err := rows.Scan(
			&e.ID, &e.RoomID, &e.TokenID, &e.Name, &e.Initiative, &e.Hidden, &e.SortOrder, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// UpdateInitiativeEntry writes the fields an edit can change. The room,
// the token it links to and its creation time are not among them: an
// entry that changed which token it stood for would be a different
// combatant.
func (s *Store) UpdateInitiativeEntry(e InitiativeEntry) error {
	_, err := s.db.Exec(
		`UPDATE initiative_entry SET name = ?, initiative = ?, hidden = ?, sort_order = ? WHERE id = ?`,
		e.Name, e.Initiative, e.Hidden, e.SortOrder, e.ID,
	)
	return err
}

// DeleteInitiativeEntry removes one entry. Idempotent, like every other
// delete here: two GMs racing on the same entry shouldn't fail the
// slower one.
func (s *Store) DeleteInitiativeEntry(id string) error {
	_, err := s.db.Exec(`DELETE FROM initiative_entry WHERE id = ?`, id)
	return err
}

// ClearInitiative empties a room's tracker and resets its turn and
// round in the same transaction — a half-cleared tracker showing round
// 7 of an encounter that no longer exists is worse than either half.
func (s *Store) ClearInitiative(roomID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM initiative_entry WHERE room_id = ?`, roomID); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE room SET initiative_round = 1, initiative_entry_id = NULL WHERE id = ?`, roomID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// SetInitiativeTurn records whose turn it is and which round it is.
func (s *Store) SetInitiativeTurn(roomID string, entryID *string, round int) error {
	_, err := s.db.Exec(
		`UPDATE room SET initiative_entry_id = ?, initiative_round = ? WHERE id = ?`, entryID, round, roomID,
	)
	return err
}

// GetInitiativeState reads the whole tracker.
//
// The current entry is validated against the list it comes back with
// rather than trusted: `initiative_entry_id` has no foreign key, so an
// entry that vanished with its token (ON DELETE CASCADE from token)
// would otherwise leave the room pointing at a combatant nobody can
// see. A dangling pointer reads as "nobody's turn", which is recoverable
// with one click on Next.
func (s *Store) GetInitiativeState(roomID string) (InitiativeState, error) {
	entries, err := s.ListInitiativeEntries(roomID)
	if err != nil {
		return InitiativeState{}, err
	}

	var currentID *string
	round := 1
	if err := s.db.QueryRow(
		`SELECT initiative_entry_id, initiative_round FROM room WHERE id = ?`, roomID,
	).Scan(&currentID, &round); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return InitiativeState{}, ErrNotFound
		}
		return InitiativeState{}, err
	}

	if currentID != nil {
		found := false
		for _, e := range entries {
			if e.ID == *currentID {
				found = true
				break
			}
		}
		if !found {
			currentID = nil
		}
	}

	return InitiativeState{Entries: entries, Round: round, CurrentEntryID: currentID}, nil
}
