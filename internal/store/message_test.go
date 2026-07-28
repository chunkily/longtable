package store

import (
	"testing"
	"time"
)

func TestInsertMessage_TextAndRoll(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	participantID := gm.ID

	if _, err := s.InsertMessage(Message{
		RoomID: room.ID, ParticipantID: &participantID, ParticipantName: gm.DisplayName,
		Kind: MessageKindText, Body: "hello",
	}); err != nil {
		t.Fatalf("InsertMessage (text): %v", err)
	}

	// created_at has nanosecond precision in the schema, but the clock
	// tick on some platforms is coarser than that; sleep a moment so the
	// two rows sort deterministically by creation time below.
	time.Sleep(2 * time.Millisecond)

	expr, breakdown, result := "2d6+3", "+2d6(4,5)+3", 12
	if _, err := s.InsertMessage(Message{
		RoomID: room.ID, ParticipantID: &participantID, ParticipantName: gm.DisplayName,
		Kind: MessageKindRoll, Body: "/roll 2d6+3",
		RollExpression: &expr, RollResult: &result, RollBreakdown: &breakdown,
	}); err != nil {
		t.Fatalf("InsertMessage (roll): %v", err)
	}

	messages, err := s.ListRecentMessages(room.ID, 50)
	if err != nil {
		t.Fatalf("ListRecentMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
	if messages[0].Kind != MessageKindRoll {
		t.Fatalf("messages[0].Kind = %q, want roll (newest first)", messages[0].Kind)
	}
	if messages[0].RollResult == nil || *messages[0].RollResult != 12 {
		t.Fatalf("RollResult = %v, want 12", messages[0].RollResult)
	}
}

func TestListRecentMessages_RespectsLimit(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Room", "GM", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	participantID := gm.ID

	for i := 0; i < 5; i++ {
		if _, err := s.InsertMessage(Message{
			RoomID: room.ID, ParticipantID: &participantID, ParticipantName: gm.DisplayName,
			Kind: MessageKindText, Body: "msg",
		}); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}

	messages, err := s.ListRecentMessages(room.ID, 3)
	if err != nil {
		t.Fatalf("ListRecentMessages: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(messages))
	}
}
