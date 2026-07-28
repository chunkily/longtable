package store

import (
	"errors"
	"testing"
)

func TestJoinRoom(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	p, err := s.JoinRoom(room.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}
	if p.Role != RolePlayer {
		t.Fatalf("role = %q, want player", p.Role)
	}

	got, err := s.GetParticipantByToken(room.ID, p.SessionToken)
	if err != nil {
		t.Fatalf("GetParticipantByToken: %v", err)
	}
	if got.ID != p.ID {
		t.Fatalf("resolved participant %q, want %q", got.ID, p.ID)
	}
}

func TestGetParticipantByToken_ScopedToRoom(t *testing.T) {
	s := newTestStore(t)

	roomA, _, err := s.CreateRoom("Room A", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	roomB, _, err := s.CreateRoom("Room B", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	p, err := s.JoinRoom(roomA.ID, "Bob")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	if _, err := s.GetParticipantByToken(roomB.ID, p.SessionToken); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (token must not resolve in a different room)", err)
	}
}

func TestGMLogin(t *testing.T) {
	s := newTestStore(t)

	room, _, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}

	p, err := s.GMLogin(room.ID, "Second GM")
	if err != nil {
		t.Fatalf("GMLogin: %v", err)
	}
	if p.Role != RoleGM {
		t.Fatalf("role = %q, want gm", p.Role)
	}
}
