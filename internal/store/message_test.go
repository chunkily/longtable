package store

import (
	"errors"
	"testing"
	"time"
)

func TestInsertMessage_TextAndRoll(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Room", "GM", "", "password")
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

	room, gm, err := s.CreateRoom("Room", "GM", "", "password")
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

func TestSoftDeleteMessage_KeepsContentAndRecordsDeleter(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	author := gm.ID
	deleter, err := s.JoinRoom(room.ID, "Bob", "")
	if err != nil {
		t.Fatalf("JoinRoom: %v", err)
	}

	expr, breakdown, result := "2d6+3", "+2d6(4,5)+3", 12
	msg, err := s.InsertMessage(Message{
		RoomID: room.ID, ParticipantID: &author, ParticipantName: gm.DisplayName,
		Kind: MessageKindRoll, Body: "/roll 2d6+3",
		RollExpression: &expr, RollResult: &result, RollBreakdown: &breakdown,
	})
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	// A GM (Bob here, playing moderator) deleting someone else's message
	// is the case that needs DeletedByParticipantID kept separately from
	// ParticipantID — the two people allowed to still see the original
	// content are not necessarily the same person.
	if err := s.SoftDeleteMessage(msg.ID, deleter.ID); err != nil {
		t.Fatalf("SoftDeleteMessage: %v", err)
	}

	got, err := s.GetMessage(msg.ID)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	// Content survives the first stage — redaction is the WS layer's job,
	// per viewer, not something the row itself does.
	if got.Body != "/roll 2d6+3" {
		t.Errorf("Body = %q, want original content kept", got.Body)
	}
	if got.RollExpression == nil || *got.RollExpression != expr {
		t.Errorf("RollExpression = %v, want kept", got.RollExpression)
	}
	if got.DeletedAt == nil {
		t.Error("DeletedAt = nil, want set")
	}
	if got.ParticipantID == nil || *got.ParticipantID != author {
		t.Errorf("ParticipantID = %v, want original author %q", got.ParticipantID, author)
	}
	if got.DeletedByParticipantID == nil || *got.DeletedByParticipantID != deleter.ID {
		t.Errorf("DeletedByParticipantID = %v, want %q", got.DeletedByParticipantID, deleter.ID)
	}
}

func TestDeleteMessage_RemovesRow(t *testing.T) {
	s := newTestStore(t)

	room, gm, err := s.CreateRoom("Room", "GM", "", "password")
	if err != nil {
		t.Fatalf("CreateRoom: %v", err)
	}
	participantID := gm.ID

	msg, err := s.InsertMessage(Message{
		RoomID: room.ID, ParticipantID: &participantID, ParticipantName: gm.DisplayName,
		Kind: MessageKindText, Body: "hello",
	})
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	if err := s.DeleteMessage(msg.ID); err != nil {
		t.Fatalf("DeleteMessage: %v", err)
	}

	if _, err := s.GetMessage(msg.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetMessage after delete: err = %v, want ErrNotFound", err)
	}

	// Deleting an already-gone message isn't an error, matching
	// DeleteDrawing — two people purging the same message at once
	// shouldn't have one of them fail.
	if err := s.DeleteMessage(msg.ID); err != nil {
		t.Fatalf("DeleteMessage (already gone): %v", err)
	}
}
