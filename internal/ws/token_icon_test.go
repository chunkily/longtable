package ws

import (
	"encoding/json"
	"testing"

	"longtable/internal/store"
)

// A token without art can wear an icon and a colour instead. Both are
// keys from fixed lists — they're drawn on everyone's screen — and they
// follow art's permissions: anyone chooses them for a token they make,
// and changing them afterwards is a GM's edit.

type iconToken struct {
	ID    string `json:"id"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

func readIconToken(t *testing.T, c *testClient, wantType string) iconToken {
	t.Helper()
	env := c.readEnvelope(t)
	if env.Type != wantType {
		t.Fatalf("type = %q, want %s", env.Type, wantType)
	}
	var tok iconToken
	if err := json.Unmarshal(env.Payload, &tok); err != nil {
		t.Fatalf("unmarshal %s: %v", wantType, err)
	}
	return tok
}

func TestTokenIcon_APlayerChoosesOneForATokenTheyMake(t *testing.T) {
	r := newPartyRoom(t)

	r.pc.send(t, "token.create", map[string]any{
		"sceneId": r.scene.ID, "name": "Familiar", "icon": "lucide:cat", "color": "teal",
	})
	got := readIconToken(t, r.pc, "token.created")
	if got.Icon != "lucide:cat" || got.Color != "teal" {
		t.Fatalf("created %+v, want the cat on teal", got)
	}
	// Everyone sees it the same way.
	if seen := readIconToken(t, r.gmc, "token.created"); seen != got {
		t.Fatalf("GM saw %+v, want %+v", seen, got)
	}

	stored, err := r.ts.store.GetToken(got.ID)
	if err != nil || stored.Icon != "lucide:cat" || stored.Color != "teal" {
		t.Fatalf("stored %+v, %v", stored, err)
	}
}

// Keys from outside the lists are refused, from anyone, on every command
// that sets them — a colour reaches a style attribute.
func TestTokenIcon_UnknownKeysAreRefused(t *testing.T) {
	r := newPartyRoom(t)
	tok := r.token(t, "Goblin", store.VisibilityVisible)

	for _, c := range []struct {
		typ     string
		payload map[string]any
		want    string
	}{
		{"token.create", map[string]any{"sceneId": r.scene.ID, "name": "X", "icon": "lucide:dragon"}, "unknown token icon"},
		{"token.create", map[string]any{"sceneId": r.scene.ID, "name": "X", "color": "red; background: url(x)"}, "unknown token colour"},
		{"token.update", map[string]any{"tokenId": tok.ID, "name": "Goblin", "icon": "sword"}, "unknown token icon"},
		{"party.memberCreate", map[string]any{"name": "Aria", "color": "chartreuse"}, "unknown token colour"},
	} {
		r.gmc.send(t, c.typ, c.payload)
		if msg := errorMessage(t, r.gmc.readEnvelope(t)); msg != c.want {
			t.Errorf("%s %v: %q, want %q", c.typ, c.payload, msg, c.want)
		}
	}
}

// Changing what a token looks like after it's made is a GM's edit, as
// changing its art is: a Player's edit to their own token keeps its
// trackers and ignores the rest.
func TestTokenIcon_OnlyAGMChangesItAfterwards(t *testing.T) {
	r := newPartyRoom(t)
	ownerID := r.player.ID
	tok, err := r.ts.store.CreateToken(store.Token{
		SceneID: r.scene.ID, Name: "Bob's Fighter", Width: 1, Height: 1,
		OwnerParticipantID: &ownerID, Icon: "lucide:sword", Color: "red",
	})
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	r.pc.send(t, "token.update", map[string]any{"tokenId": tok.ID, "icon": "lucide:crown", "color": "gold"})
	if got := readIconToken(t, r.pc, "token.updated"); got.Icon != "lucide:sword" || got.Color != "red" {
		t.Fatalf("a Player changed their token's look: %+v", got)
	}
	r.gmc.readEnvelope(t)

	r.gmc.send(t, "token.update", map[string]any{
		"tokenId": tok.ID, "name": "Bob's Fighter", "width": 1, "height": 1,
		"ownerParticipantId": ownerID, "icon": "lucide:crown", "color": "gold",
	})
	if got := readIconToken(t, r.gmc, "token.updated"); got.Icon != "lucide:crown" || got.Color != "gold" {
		t.Fatalf("GM edit came back %+v", got)
	}
}

// A party member's icon and colour are what its copies wear.
func TestTokenIcon_PartyCopiesWearTheMembersIcon(t *testing.T) {
	r := newPartyRoom(t)
	members := r.member(t, map[string]any{"name": "Wolf", "icon": "lucide:paw-print", "color": "grey"})
	if members[0].Name != "Wolf" {
		t.Fatalf("roster = %+v", members)
	}

	r.add(t, r.next.ID)
	if got := readIconToken(t, r.gmc, "token.created"); got.Icon != "lucide:paw-print" || got.Color != "grey" {
		t.Fatalf("copy wears %+v, want the member's paw print on grey", got)
	}
}
