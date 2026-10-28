package store

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// PartyMember is one character in a room's party, as the GM defined it.
//
// Not a token, on purpose. The party was first a mark on tokens, with the
// newest copy of each marked character standing in for it — which meant a
// character's art lived on whichever map it was last edited on, and
// configuring the party meant hunting through scenes. Here it is defined
// once, in the GM's Party dialog, and the tokens on maps are copies made
// from it (each carrying the member's id as its PartyKey).
type PartyMember struct {
	ID                 string
	RoomID             string
	Name               string
	ImageAssetID       *string
	Width, Height      float64
	OwnerParticipantID *string
	// Attending is whether they're at the table tonight; party.add skips
	// anyone who isn't. New members are attending.
	Attending bool
	CreatedAt string
}

const partyMemberColumns = `id, room_id, name, image_asset_id, width, height, owner_participant_id, attending, created_at`

func scanPartyMember(row rowScanner) (PartyMember, error) {
	var m PartyMember
	err := row.Scan(&m.ID, &m.RoomID, &m.Name, &m.ImageAssetID, &m.Width, &m.Height, &m.OwnerParticipantID, &m.Attending, &m.CreatedAt)
	return m, err
}

// ListPartyMembers returns a room's party in the order the GM added it,
// which is the order the dialog lists it and party.add puts it down.
// rowid breaks a tie between two members added in the same instant.
func (s *Store) ListPartyMembers(roomID string) ([]PartyMember, error) {
	rows, err := s.db.Query(
		`SELECT `+partyMemberColumns+` FROM party_member WHERE room_id = ? ORDER BY created_at, rowid`, roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []PartyMember{}
	for rows.Next() {
		m, err := scanPartyMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// CountPartyMembers is how big a room's party is, for the cap on it.
func (s *Store) CountPartyMembers(roomID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM party_member WHERE room_id = ?`, roomID).Scan(&n)
	return n, err
}

// CreatePartyMember adds a character to the party. The id and creation
// time are the store's; whatever the caller put there is ignored.
func (s *Store) CreatePartyMember(m PartyMember) (PartyMember, error) {
	m.ID = uuid.NewString()
	m.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(
		`INSERT INTO party_member (`+partyMemberColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.RoomID, m.Name, m.ImageAssetID, m.Width, m.Height, m.OwnerParticipantID, m.Attending, m.CreatedAt,
	)
	if err != nil {
		return PartyMember{}, err
	}
	return m, nil
}

// UpdatePartyMember rewrites a member's details, keyed by its id *and*
// its room — so a member id from another room answers ErrNotFound, the
// same as one that doesn't exist, and can't be probed for.
//
// It changes the member and nothing on any map: copies already put down
// are tokens in their own right, and the GM edits those as tokens.
func (s *Store) UpdatePartyMember(m PartyMember) error {
	res, err := s.db.Exec(
		`UPDATE party_member SET name = ?, image_asset_id = ?, width = ?, height = ?, owner_participant_id = ?, attending = ?
		 WHERE id = ? AND room_id = ?`,
		m.Name, m.ImageAssetID, m.Width, m.Height, m.OwnerParticipantID, m.Attending, m.ID, m.RoomID,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res)
}

// DeletePartyMember takes a character out of the party, room-scoped like
// UpdatePartyMember. Its copies stay on their maps as ordinary tokens —
// leaving the party is not leaving the scene — and the PartyKey they
// carry simply stops matching anything.
func (s *Store) DeletePartyMember(roomID, id string) error {
	res, err := s.db.Exec(`DELETE FROM party_member WHERE id = ? AND room_id = ?`, id, roomID)
	if err != nil {
		return err
	}
	return requireOneRow(res)
}

func requireOneRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
