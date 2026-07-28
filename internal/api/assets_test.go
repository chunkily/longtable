package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

	resp := uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "map.png", []byte("fake png bytes"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var asset assetPayloadT
	decodeJSONBody(t, resp, &asset)
	if asset.ID == "" {
		t.Fatal("expected an asset ID")
	}
}

func TestUploadAsset_DuplicateContentReusesAsset(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)
	content := []byte("identical bytes")

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

	resp := uploadTestAsset(t, srv, created.RoomSlug, "", "map.png", []byte("bytes"))
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

func TestServeAsset_RoundTrip(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createTestRoom(t, srv)
	content := []byte("round trip bytes")

	uploadResp := uploadTestAsset(t, srv, created.RoomSlug, created.SessionToken, "map.png", content)
	var asset assetPayloadT
	decodeJSONBody(t, uploadResp, &asset)

	resp, err := http.Get(srv.URL + "/api/assets/" + asset.ID)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatal("served content does not match uploaded content")
	}
}
