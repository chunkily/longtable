package api

import (
	"bytes"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"longtable/internal/store"

	"github.com/gen2brain/webp"
)

// What a room's copy of an asset carries beyond the pixels: the name it
// goes by, the credit, and — for a map — the square size measured when
// it was aligned. Helpers live in assets_test.go.

// The assets page sends all of an asset's details with the file, because
// nothing is stored until it does — there is no half-finished library
// entry to go back and fill in.
func TestUploadAsset_RecordsNameAndGridSize(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := uploadWithFields(t, srv, created.RoomSlug, created.SessionToken, "dungeon_final_v2.png",
		testPNG(t, 64, 64, color.RGBA{R: 40, G: 40, B: 40, A: 255}),
		map[string]string{"name": "  Sunless citadel  ", "gridSize": "70"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var uploaded assetPayloadT
	decodeJSONBody(t, resp, &uploaded)
	if uploaded.Name != "Sunless citadel" {
		t.Fatalf("name = %q, want it trimmed to %q", uploaded.Name, "Sunless citadel")
	}
	if uploaded.GridSize == nil || *uploaded.GridSize != 70 {
		t.Fatalf("gridSize = %v, want 70", uploaded.GridSize)
	}

	var library []assetPayloadT
	decodeJSONBody(t, listRoomAssets(t, srv, created.RoomSlug, created.SessionToken), &library)
	if len(library) != 1 || library[0].Name != "Sunless citadel" {
		t.Fatalf("library = %+v, want the named asset", library)
	}
	if library[0].GridSize == nil || *library[0].GridSize != 70 {
		t.Fatalf("library gridSize = %v, want 70", library[0].GridSize)
	}
}

// Tokens have no grid, and a map can be added without aligning it, so an
// upload carrying neither name nor grid size is the ordinary case rather
// than an error. The name still has to come out non-empty: an entry with
// nothing to search on is one nobody finds.
func TestUploadAsset_DefaultsNameFromFilename(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var uploaded assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "goblin archer.png",
		testPNG(t, 16, 16, color.RGBA{R: 200, G: 30, B: 30, A: 255})), &uploaded)

	if uploaded.Name != "goblin archer" {
		t.Fatalf("name = %q, want the filename without its extension", uploaded.Name)
	}
	if uploaded.GridSize != nil {
		t.Fatalf("gridSize = %d, want null for an unaligned upload", *uploaded.GridSize)
	}
}

// The kind is what sorts the library into its two tabs, so it travels
// with the upload like everything else the assets page collects.
func TestUploadAsset_RecordsTheKind(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var asMap assetPayloadT
	decodeJSONBody(t, uploadWithFields(t, srv, created.RoomSlug, created.SessionToken, "keep.png",
		testPNG(t, 32, 32, color.RGBA{R: 30, G: 90, B: 30, A: 255}),
		map[string]string{"kind": "map"}), &asMap)
	if asMap.Kind != store.AssetKindMap {
		t.Fatalf("kind = %q, want map", asMap.Kind)
	}

	// Silence is a token: it's what most art is, and a client that predates
	// the split has to keep working.
	var unstated assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "goblin.png",
		testPNG(t, 16, 16, color.RGBA{R: 180, G: 40, B: 40, A: 255})), &unstated)
	if unstated.Kind != store.AssetKindToken {
		t.Fatalf("kind = %q for an upload that didn't say, want token", unstated.Kind)
	}

	// Anything else is a client bug, and filing the art under a kind nobody
	// asked for would hide it in the wrong tab.
	resp := uploadWithFields(t, srv, created.RoomSlug, created.SessionToken, "what.png",
		testPNG(t, 8, 8, color.RGBA{R: 1, G: 1, B: 1, A: 255}),
		map[string]string{"kind": "portrait"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d for an unknown kind, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestUploadAsset_RejectsNonsenseGridFigures(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)
	content := testPNG(t, 16, 16, color.RGBA{R: 1, G: 2, B: 3, A: 255})

	for _, tc := range []struct {
		name   string
		fields map[string]string
	}{
		{"non-numeric size", map[string]string{"gridSize": "seventy"}},
		{"size below the floor", map[string]string{"gridSize": "2"}},
		{"size past the ceiling", map[string]string{"gridSize": "9000"}},
		{"offset past the ceiling", map[string]string{"gridOffsetX": "100000"}},
	} {
		resp := uploadWithFields(t, srv, created.RoomSlug, created.SessionToken, "m.png", content, tc.fields)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", tc.name, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// The alignment lives in the pixels: a map padded on upload is served
// already aligned, so nothing downstream has to know an offset ever
// existed. That is the whole argument of the grid-offset story.
func TestUploadAsset_BakesGridOffsetIntoTheStoredImage(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var uploaded assetPayloadT
	decodeJSONBody(t, uploadWithFields(t, srv, created.RoomSlug, created.SessionToken, "map.png",
		testPNG(t, 100, 80, color.RGBA{R: 20, G: 120, B: 20, A: 255}),
		map[string]string{"gridOffsetX": "12", "gridOffsetY": "-5"}), &uploaded)

	served := getAssetBytes(t, srv, uploaded.ID)
	cfg, err := webp.DecodeConfig(bytes.NewReader(served))
	if err != nil {
		t.Fatalf("served bytes are not webp: %v", err)
	}
	// Padded 12px on the left, cropped 5px off the top.
	if cfg.Width != 112 || cfg.Height != 75 {
		t.Fatalf("served image is %dx%d, want 112x75", cfg.Width, cfg.Height)
	}
}

func patchRoomAsset(t *testing.T, srv *httptest.Server, slug, token, assetID, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodPatch,
		srv.URL+"/api/rooms/"+slug+"/assets/"+assetID, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// Names default from filenames, so a first pass through a library is
// full of things nobody would search for. Renaming has to work without
// re-uploading the file.
func TestUpdateRoomAsset_RenamesAndRecredits(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var uploaded assetPayloadT
	decodeJSONBody(t, uploadWithFields(t, srv, created.RoomSlug, created.SessionToken, "img_4471.png",
		testPNG(t, 24, 24, color.RGBA{R: 60, G: 60, B: 160, A: 255}),
		map[string]string{"attribution": "by Alice"}), &uploaded)

	resp := patchRoomAsset(t, srv, created.RoomSlug, created.SessionToken, uploaded.ID,
		`{"name":"Ruined keep","attribution":"by Alice, CC-BY"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var updated assetPayloadT
	decodeJSONBody(t, resp, &updated)
	if updated.Name != "Ruined keep" || updated.Attribution != "by Alice, CC-BY" {
		t.Fatalf("updated = %+v, want the new name and credit", updated)
	}

	var library []assetPayloadT
	decodeJSONBody(t, listRoomAssets(t, srv, created.RoomSlug, created.SessionToken), &library)
	if library[0].Name != "Ruined keep" {
		t.Fatalf("library name = %q, want the rename to have stuck", library[0].Name)
	}
}

// Unlike an upload, where an absent credit means "I had nothing to add",
// a form submission with an empty credit means "clear it".
func TestUpdateRoomAsset_ClearsAttributionWhenAsked(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var uploaded assetPayloadT
	decodeJSONBody(t, uploadWithAttribution(t, srv, created.RoomSlug, created.SessionToken, "m.png",
		testPNG(t, 8, 8, color.RGBA{R: 9, G: 9, B: 9, A: 255}), "by Bob"), &uploaded)

	var updated assetPayloadT
	decodeJSONBody(t, patchRoomAsset(t, srv, created.RoomSlug, created.SessionToken, uploaded.ID,
		`{"name":"Map","attribution":""}`), &updated)
	if updated.Attribution != "" {
		t.Fatalf("attribution = %q, want it cleared", updated.Attribution)
	}
}

// Whether a picture is a token or a map is this room's opinion of it,
// not a fact about the pixels — so unlike the image and the measured
// grid, it can be changed afterwards. It's also the only way to fix what
// the migration guessed for a library that predates the split.
func TestUpdateRoomAsset_ReclassifiesAndLeavesTheKindAloneWhenUnstated(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var uploaded assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "tavern.png",
		testPNG(t, 40, 40, color.RGBA{R: 120, G: 80, B: 20, A: 255})), &uploaded)

	var reclassified assetPayloadT
	decodeJSONBody(t, patchRoomAsset(t, srv, created.RoomSlug, created.SessionToken, uploaded.ID,
		`{"name":"Tavern","attribution":"","kind":"map"}`), &reclassified)
	if reclassified.Kind != store.AssetKindMap {
		t.Fatalf("kind = %q, want map", reclassified.Kind)
	}

	// An older client's PATCH carries no kind. An empty credit means
	// "clear it", but there is no third kind for an empty one to mean, so
	// silence there can only leave the asset where it is.
	var renamed assetPayloadT
	decodeJSONBody(t, patchRoomAsset(t, srv, created.RoomSlug, created.SessionToken, uploaded.ID,
		`{"name":"The tavern","attribution":""}`), &renamed)
	if renamed.Kind != store.AssetKindMap {
		t.Fatalf("kind = %q after a patch that omitted it, want map", renamed.Kind)
	}

	resp := patchRoomAsset(t, srv, created.RoomSlug, created.SessionToken, uploaded.ID,
		`{"name":"The tavern","kind":"scenery"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d for an unknown kind, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestUpdateRoomAsset_RejectsAnEmptyName(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var uploaded assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "m.png",
		testPNG(t, 8, 8, color.RGBA{R: 7, G: 7, B: 7, A: 255})), &uploaded)

	resp := patchRoomAsset(t, srv, created.RoomSlug, created.SessionToken, uploaded.ID,
		`{"name":"   ","attribution":""}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

// An asset in another room and an asset that doesn't exist give the same
// answer, so a 404 can't be used to probe what exists elsewhere.
func TestUpdateRoomAsset_WontReachIntoAnotherRoomsLibrary(t *testing.T) {
	srv, _ := newTestServer(t)
	roomA := createTestRoom(t, srv)
	roomB := createTestRoom(t, srv)

	var inA assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, roomA.RoomSlug, roomA.SessionToken, "m.png",
		testPNG(t, 12, 12, color.RGBA{R: 5, G: 90, B: 5, A: 255})), &inA)

	// Room B holds a valid session and a real asset ID — just not one of
	// theirs.
	resp := patchRoomAsset(t, srv, roomB.RoomSlug, roomB.SessionToken, inA.ID, `{"name":"Mine now"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("foreign asset: status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()

	missing := patchRoomAsset(t, srv, roomB.RoomSlug, roomB.SessionToken,
		"11111111-1111-1111-1111-111111111111", `{"name":"Ghost"}`)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing asset: status = %d, want 404", missing.StatusCode)
	}
	missing.Body.Close()

	// A's own copy is untouched.
	var library []assetPayloadT
	decodeJSONBody(t, listRoomAssets(t, srv, roomA.RoomSlug, roomA.SessionToken), &library)
	if library[0].Name != "m" {
		t.Fatalf("room A name = %q, want it unchanged", library[0].Name)
	}
}

func deleteRoomAsset(t *testing.T, srv *httptest.Server, slug, token, assetID string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/rooms/"+slug+"/assets/"+assetID, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// Taking a picture off this room's shelf leaves the file alone: it's
// content-addressed and shared, the room can put it back by adding it
// again, and the bytes stay servable for anything already using them.
func TestRemoveRoomAsset_ClearsTheShelfButNotTheFile(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var uploaded assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "keep.png",
		testPNG(t, 20, 20, color.RGBA{R: 80, G: 20, B: 90, A: 255})), &uploaded)

	resp := deleteRoomAsset(t, srv, created.RoomSlug, created.SessionToken, uploaded.ID)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	var library []assetPayloadT
	decodeJSONBody(t, listRoomAssets(t, srv, created.RoomSlug, created.SessionToken), &library)
	if len(library) != 0 {
		t.Fatalf("library = %+v, want it empty", library)
	}

	// Still served by ID, which is what keeps a scene or token that was
	// already using it from turning into a broken image.
	if got := len(getAssetBytes(t, srv, uploaded.ID)); got == 0 {
		t.Fatal("asset bytes are gone, want them still served")
	}

	// Removing it twice is a 404, not a silent success — the same answer
	// another room's asset gets.
	again := deleteRoomAsset(t, srv, created.RoomSlug, created.SessionToken, uploaded.ID)
	if again.StatusCode != http.StatusNotFound {
		t.Fatalf("second removal: status = %d, want 404", again.StatusCode)
	}
	again.Body.Close()
}

func TestRemoveRoomAsset_WontReachIntoAnotherRoomsLibrary(t *testing.T) {
	srv, _ := newTestServer(t)
	roomA := createTestRoom(t, srv)
	roomB := createTestRoom(t, srv)

	var inA assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, roomA.RoomSlug, roomA.SessionToken, "m.png",
		testPNG(t, 12, 12, color.RGBA{R: 5, G: 90, B: 5, A: 255})), &inA)

	resp := deleteRoomAsset(t, srv, roomB.RoomSlug, roomB.SessionToken, inA.ID)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("foreign asset: status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()

	unauthenticated := deleteRoomAsset(t, srv, roomA.RoomSlug, "", inA.ID)
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no session: status = %d, want 401", unauthenticated.StatusCode)
	}
	unauthenticated.Body.Close()

	// A's copy survived both attempts.
	var library []assetPayloadT
	decodeJSONBody(t, listRoomAssets(t, srv, roomA.RoomSlug, roomA.SessionToken), &library)
	if len(library) != 1 {
		t.Fatalf("room A library has %d entries, want 1", len(library))
	}
}

func TestUpdateRoomAsset_RequiresASessionForThatRoom(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var uploaded assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "m.png",
		testPNG(t, 8, 8, color.RGBA{R: 3, G: 3, B: 3, A: 255})), &uploaded)

	resp := patchRoomAsset(t, srv, created.RoomSlug, "", uploaded.ID, `{"name":"Anon"}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()
}
