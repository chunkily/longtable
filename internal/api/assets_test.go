package api

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gen2brain/webp"
)

// testPNG builds a real image, because uploads are now decoded and
// re-encoded — arbitrary bytes are rejected on purpose, so a fixture
// has to be a genuine picture. The fill colour makes two fixtures
// distinguishable, which is what the dedup tests turn on.
func testPNG(t *testing.T, w, h int, fill color.RGBA) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, fill)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func uploadTestAsset(t *testing.T, srv *httptest.Server, slug, token, filename string, content []byte) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/rooms/"+slug+"/assets", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func TestUploadAsset_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "map.png",
		testPNG(t, 32, 32, color.RGBA{R: 90, G: 140, B: 90, A: 255}))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var asset assetPayloadT
	decodeJSONBody(t, resp, &asset)
	if asset.ID == "" {
		t.Fatal("expected an asset ID")
	}
	// Whatever was uploaded, what is stored is WebP — the filename is
	// rewritten to match so the library doesn't claim otherwise.
	if asset.MimeType != "image/webp" {
		t.Fatalf("mimeType = %q, want image/webp", asset.MimeType)
	}
	if asset.Filename != "map.webp" {
		t.Fatalf("filename = %q, want map.webp", asset.Filename)
	}
	if asset.Flattened {
		t.Fatal("a still image should not be reported as flattened")
	}
}

// The story's central promise: the bytes served are never the bytes
// uploaded, so anything smuggled in alongside the pixels is gone.
func TestUploadAsset_ReencodesAndStripsNonPixelData(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	payload := []byte("<script>alert(1)</script>")
	uploaded := append(testPNG(t, 24, 24, color.RGBA{R: 10, G: 20, B: 200, A: 255}), payload...)

	resp := uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "sneaky.png", uploaded)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var asset assetPayloadT
	decodeJSONBody(t, resp, &asset)

	served := getAssetBytes(t, srv, asset.ID)
	if bytes.Contains(served, payload) {
		t.Fatal("appended payload is being served back")
	}
	if bytes.Equal(served, uploaded) {
		t.Fatal("the uploaded bytes are being served verbatim")
	}
	if _, err := webp.DecodeConfig(bytes.NewReader(served)); err != nil {
		t.Fatalf("served bytes are not webp: %v", err)
	}
}

func TestUploadAsset_RejectsWhatIsNotAnImage(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "notes.txt",
		[]byte("this is a text file wearing a .png costume"))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestUploadAsset_DuplicateContentReusesAsset(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)
	content := testPNG(t, 40, 40, color.RGBA{R: 200, G: 30, B: 30, A: 255})

	first := uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "a.png", content)
	var firstAsset assetPayloadT
	decodeJSONBody(t, first, &firstAsset)

	second := uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "b.png", content)
	var secondAsset assetPayloadT
	decodeJSONBody(t, second, &secondAsset)

	if firstAsset.ID != secondAsset.ID {
		t.Fatalf("uploading identical content twice produced different asset IDs: %q vs %q", firstAsset.ID, secondAsset.ID)
	}
}

func TestUploadAsset_MissingSessionToken(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := uploadTestAsset(t, srv, created.RoomSlug, "", "map.png",
		testPNG(t, 8, 8, color.RGBA{A: 255}))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUploadAsset_MissingFileField(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("notfile", "x"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/rooms/"+created.RoomSlug+"/assets", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+created.SessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestServeAsset_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/assets/nosuchid")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func getAssetBytes(t *testing.T, srv *httptest.Server, id string) []byte {
	t.Helper()

	resp, err := http.Get(srv.URL + "/api/assets/" + id)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return got
}

// Served bytes can't be compared against what was uploaded any more —
// they're deliberately different — so this checks the picture made it
// through: same dimensions, decodable as WebP, served as WebP.
func TestServeAsset_RoundTrip(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	uploadResp := uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "map.png",
		testPNG(t, 48, 36, color.RGBA{R: 120, G: 90, B: 200, A: 255}))
	var asset assetPayloadT
	decodeJSONBody(t, uploadResp, &asset)

	served := getAssetBytes(t, srv, asset.ID)
	cfg, err := webp.DecodeConfig(bytes.NewReader(served))
	if err != nil {
		t.Fatalf("served bytes are not webp: %v", err)
	}
	if cfg.Width != 48 || cfg.Height != 36 {
		t.Fatalf("served image is %dx%d, want 48x36", cfg.Width, cfg.Height)
	}
	if int64(len(served)) != asset.ByteSize {
		t.Fatalf("served %d bytes but the asset records %d", len(served), asset.ByteSize)
	}
}

// --- room libraries ---

func listRoomAssets(t *testing.T, srv *httptest.Server, slug, token string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/rooms/"+slug+"/assets", nil)
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

func uploadWithAttribution(t *testing.T, srv *httptest.Server, slug, token, filename string, content []byte, attribution string) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	if err := writer.WriteField("attribution", attribution); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/rooms/"+slug+"/assets", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// An upload is also a library addition — that's what makes "upload new"
// and "pick from library" the same gesture from the second time onwards.
func TestUploadAsset_AddsToTheRoomLibraryWithAttribution(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := uploadWithAttribution(t, srv, created.RoomSlug, created.SessionToken, "tavern.png",
		testPNG(t, 16, 16, color.RGBA{R: 30, G: 90, B: 30, A: 255}), "  by Alice, CC-BY  ")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var uploaded assetPayloadT
	decodeJSONBody(t, resp, &uploaded)
	if uploaded.Attribution != "by Alice, CC-BY" {
		t.Fatalf("attribution = %q, want it trimmed to %q", uploaded.Attribution, "by Alice, CC-BY")
	}

	listResp := listRoomAssets(t, srv, created.RoomSlug, created.SessionToken)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", listResp.StatusCode)
	}
	var library []assetPayloadT
	decodeJSONBody(t, listResp, &library)

	if len(library) != 1 {
		t.Fatalf("library has %d entries, want 1", len(library))
	}
	if library[0].ID != uploaded.ID {
		t.Fatalf("library holds %q, want the uploaded %q", library[0].ID, uploaded.ID)
	}
	if library[0].Attribution != "by Alice, CC-BY" {
		t.Fatalf("library attribution = %q", library[0].Attribution)
	}
}

// A room's library is the list of everything that room has, so handing it
// to anyone holding the slug would leak the contents of a room they
// can't otherwise see.
func TestListRoomAssets_RequiresASessionForThatRoom(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)
	other := createTestRoom(t, srv)

	if resp := listRoomAssets(t, srv, created.RoomSlug, ""); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token: status = %d, want 401", resp.StatusCode)
	}
	// A valid session, but for a different room.
	if resp := listRoomAssets(t, srv, created.RoomSlug, other.SessionToken); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("foreign token: status = %d, want 401", resp.StatusCode)
	}
}

// The two halves of the model at once: one stored file shared between
// rooms, but each room only sees what it added itself.
func TestRoomLibraries_ShareStorageWithoutSharingVisibility(t *testing.T) {
	srv, _ := newTestServer(t)
	roomA := createTestRoom(t, srv)
	roomB := createTestRoom(t, srv)
	content := testPNG(t, 20, 20, color.RGBA{R: 10, G: 10, B: 220, A: 255})

	var inA assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, roomA.RoomSlug, roomA.SessionToken, "map.png", content), &inA)

	// Room B hasn't uploaded anything yet.
	var libraryB []assetPayloadT
	decodeJSONBody(t, listRoomAssets(t, srv, roomB.RoomSlug, roomB.SessionToken), &libraryB)
	if len(libraryB) != 0 {
		t.Fatalf("room B library = %+v, want empty", libraryB)
	}

	// The same bytes uploaded to room B resolve to the same asset — one
	// stored file — and now appear in B's library too.
	var inB assetPayloadT
	decodeJSONBody(t, uploadTestAsset(t, srv, roomB.RoomSlug, roomB.SessionToken, "map.png", content), &inB)
	if inA.ID != inB.ID {
		t.Fatalf("identical content produced two assets: %q and %q", inA.ID, inB.ID)
	}

	decodeJSONBody(t, listRoomAssets(t, srv, roomB.RoomSlug, roomB.SessionToken), &libraryB)
	if len(libraryB) != 1 || libraryB[0].ID != inA.ID {
		t.Fatalf("room B library = %+v, want the shared asset", libraryB)
	}
}

func TestListRoomAssets_EmptyLibraryIsAnEmptyList(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)

	resp := listRoomAssets(t, srv, created.RoomSlug, created.SessionToken)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	resp.Body.Close()

	// [] rather than null, so the client can iterate without a nil check.
	if strings.TrimSpace(string(body)) != "[]" {
		t.Fatalf("body = %q, want []", body)
	}
}
