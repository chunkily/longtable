package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"longtable/internal/imageproc"
	"longtable/internal/store"
)

const maxAssetUploadBytes = 25 << 20 // 25 MiB — generous for map/token art

// maxAttributionLength bounds the free-text credit field. Long enough
// for a name, a licence and a URL; short enough that it can't be used as
// storage. Truncated rather than rejected, since losing the tail of a
// credit is a smaller harm than losing the upload it came with.
const maxAttributionLength = 500

// maxAssetNameLength bounds the display name. Shorter than the credit
// because this one has to fit under a thumbnail.
const maxAssetNameLength = 120

// Bounds on the grid figures an upload may carry. The size is pixels per
// square, and the range covers everything from a tiny web-resolution map
// to a poster scan. The offset is what gets padded onto the top-left to
// bring the art's squares onto the grid, so it is only ever a fraction
// of a square — the ceiling is well clear of any real square size, and
// is there to stop a hand-rolled request padding a map into a
// multi-gigabyte canvas one pixel at a time.
const (
	minGridSize   = 8
	maxGridSize   = 1024
	maxGridOffset = 4096
)

type assetPayloadT struct {
	ID string `json:"id"`
	// Name is what this room calls the asset, and what its library and
	// pickers show. Defaults to the filename minus its extension.
	Name     string `json:"name"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	ByteSize int64  `json:"byteSize"`
	// Attribution is whatever free text the uploader gave as credit or
	// licence for this room's copy of the asset. Empty when none was
	// given, which is the common case.
	Attribution string `json:"attribution"`
	// Kind is "token" or "map" — which half of the library this belongs
	// to, and which pickers offer it first.
	Kind store.AssetKind `json:"kind"`
	// GridSize is the map's pixels per square, measured when it was
	// aligned on the assets page, so creating a scene from it can default
	// to the right number instead of asking someone to guess it again.
	// Null for anything nobody aligned.
	GridSize *int `json:"gridSize"`
	// Flattened says an animated upload was accepted as a still image, so
	// the uploader can be told rather than left wondering why their
	// goblin stopped moving. Absent for the ordinary case.
	Flattened bool `json:"flattened,omitempty"`
}

func assetPayload(a store.Asset) assetPayloadT {
	return assetPayloadT{
		ID:       a.ID,
		Name:     store.DisplayNameFromFilename(a.Filename),
		Filename: a.Filename,
		MimeType: a.MimeType,
		ByteSize: a.ByteSize,
	}
}

func libraryAssetPayload(a store.LibraryAsset) assetPayloadT {
	payload := assetPayload(a.Asset)
	payload.Name = a.Name
	payload.Attribution = a.Attribution
	payload.Kind = a.Kind
	payload.GridSize = a.GridSize
	return payload
}

// uploadAsset accepts a multipart file upload scoped to a room (any
// authenticated participant of that room may upload — image reuse
// across rooms happens automatically via content hashing regardless of
// who uploaded it first). Auth is a bearer session token rather than
// the room/token query params the WS endpoint uses, since this is a
// plain POST.
func (srv *Server) uploadAsset(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}

	if !srv.requireParticipant(w, r, room) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAssetUploadBytes)
	if err := r.ParseMultipartForm(maxAssetUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "upload too large or malformed (25MiB max)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing \"file\" field")
		return
	}
	defer file.Close()

	offset, ok := parseGridOffset(w, r)
	if !ok {
		return
	}

	// Re-encoded before anything else looks at it: from here on the only
	// bytes in play are ones this program produced from the decoded
	// pixels. See internal/imageproc for why that matters. Grid alignment
	// rides along here rather than being stored beside the image, so the
	// pixels that get served are already aligned — see the Offset doc.
	image, err := imageproc.ReencodeOffset(file, offset)
	if err != nil {
		switch {
		case errors.Is(err, imageproc.ErrUnsupportedFormat):
			writeError(w, http.StatusBadRequest,
				"that file isn't a PNG, JPEG, WebP or GIF image")
		case errors.Is(err, imageproc.ErrTooLarge):
			writeError(w, http.StatusBadRequest, "image dimensions are too large")
		case errors.Is(err, imageproc.ErrBadOffset):
			writeError(w, http.StatusBadRequest, "that grid alignment would crop the whole image away")
		default:
			slog.Error("api: re-encode upload failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to process image")
		}
		return
	}

	asset, err := srv.storeImage(image, header.Filename)
	if err != nil {
		slog.Error("api: upload asset failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to store upload")
		return
	}

	// Every upload joins the uploading room's library, whether or not it
	// turned out to be a file the server already had. That's what makes
	// "upload new" and "pick from library" the same gesture from the
	// second time onwards, and it's why dedup can be global without one
	// room's art showing up in another's picker.
	attribution := clampText(r.FormValue("attribution"), maxAttributionLength)
	name := clampText(r.FormValue("name"), maxAssetNameLength)
	if name == "" {
		// The page fills this in from the filename, so an empty one means
		// an older client or a hand-rolled request rather than someone
		// clearing the field. Falling back keeps the library from growing
		// entries with nothing to search on.
		name = store.DisplayNameFromFilename(asset.Filename)
	}
	gridSize, ok := parseGridSize(w, r)
	if !ok {
		return
	}
	kind, ok := parseAssetKind(w, r.FormValue("kind"))
	if !ok {
		return
	}
	if err := srv.store.AddAssetToRoom(room.ID, asset.ID, name, attribution, kind, gridSize); err != nil {
		slog.Error("api: add asset to room library failed", "error", err)
		writeError(w, http.StatusInternalServerError, "stored the image, but failed to add it to the library")
		return
	}

	// Read back rather than echoing what was sent: an upload of a file
	// the room already had keeps the earlier name, credit and grid size
	// wherever this request left one out, and the client should be
	// looking at what the library actually holds.
	entry, err := srv.store.GetRoomAsset(room.ID, asset.ID)
	if err != nil {
		slog.Error("api: read back library entry failed", "error", err)
		writeError(w, http.StatusInternalServerError, "stored the image, but failed to read it back")
		return
	}

	payload := libraryAssetPayload(entry)
	payload.Flattened = image.Animated
	writeJSON(w, http.StatusCreated, payload)
}

// updateRoomAsset renames, re-credits and reclassifies a room's copy of
// an asset. It deliberately can't touch the image or its grid size: both
// live in the stored pixels, and pixels are what an asset's identity is
// made of, so "edit" there would mean making a different asset. The
// kind isn't in the pixels — it's what this room decided the picture is
// for — so that one moves.
func (srv *Server) updateRoomAsset(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	if !srv.requireParticipant(w, r, room) {
		return
	}

	var body struct {
		Name        string `json:"name"`
		Attribution string `json:"attribution"`
		Kind        string `json:"kind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := clampText(body.Name, maxAssetNameLength)
	if name == "" {
		writeError(w, http.StatusBadRequest, "a name is required")
		return
	}
	kind, ok := parseAssetKind(w, body.Kind)
	if !ok {
		return
	}

	assetID := r.PathValue("id")
	err := srv.store.UpdateRoomAsset(
		room.ID, assetID, name, clampText(body.Attribution, maxAttributionLength), kind)
	if errors.Is(err, store.ErrNotFound) {
		// Same answer for "no such asset" and "that asset belongs to
		// another room", so the failure can't be used to probe what exists
		// elsewhere — the pattern the WS layer already uses.
		writeError(w, http.StatusNotFound, "no such asset in this room's library")
		return
	}
	if err != nil {
		slog.Error("api: update room asset failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update the asset")
		return
	}

	entry, err := srv.store.GetRoomAsset(room.ID, assetID)
	if err != nil {
		slog.Error("api: read back updated asset failed", "error", err)
		writeError(w, http.StatusInternalServerError, "saved the change, but failed to read it back")
		return
	}
	writeJSON(w, http.StatusOK, libraryAssetPayload(entry))
}

// removeRoomAsset takes an asset off this room's shelf.
//
// Open to any room member, like uploading and renaming: a room's library
// is shared workspace rather than one person's, and the damage is
// bounded — the file itself is untouched, anything already on the table
// keeps working, and adding the image again puts it straight back. That
// last part is what makes this different from deleting a scene, which is
// GM-only and unrecoverable.
func (srv *Server) removeRoomAsset(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	if !srv.requireParticipant(w, r, room) {
		return
	}

	err := srv.store.RemoveAssetFromRoom(room.ID, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no such asset in this room's library")
		return
	}
	if err != nil {
		slog.Error("api: remove room asset failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove the asset")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clampText trims a free-text field and cuts it to length. Truncated
// rather than rejected, since losing the tail of a credit is a smaller
// harm than losing the upload it came with.
func clampText(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) > max {
		s = s[:max]
	}
	return s
}

// parseAssetKind reads a "token"/"map" field. Absent stays absent
// rather than becoming a default here: the store reads the empty string
// as "not supplied" and keeps whatever a room already decided, which is
// what stops an old client's silent upload from reclassifying a map it
// knew nothing about. Anything else is a client bug, and answering it
// with a 400 is kinder than filing the art under a kind nobody asked
// for.
func parseAssetKind(w http.ResponseWriter, raw string) (store.AssetKind, bool) {
	kind := store.AssetKind(strings.TrimSpace(raw))
	if kind == "" || kind.Valid() {
		return kind, true
	}
	writeError(w, http.StatusBadRequest, `an asset is either a "token" or a "map"`)
	return "", false
}

// parseGridSize reads the measured pixels-per-square off an upload.
// Absent is the common case — tokens have no grid, and a map can be
// added without aligning it — and is not an error.
func parseGridSize(w http.ResponseWriter, r *http.Request) (*int, bool) {
	raw := strings.TrimSpace(r.FormValue("gridSize"))
	if raw == "" {
		return nil, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < minGridSize || n > maxGridSize {
		writeError(w, http.StatusBadRequest, "grid size must be a whole number of pixels")
		return nil, false
	}
	return &n, true
}

// parseGridOffset reads the alignment padding off an upload. The client
// sends what it wants added to the top and left; negative crops instead.
func parseGridOffset(w http.ResponseWriter, r *http.Request) (imageproc.Offset, bool) {
	var offset imageproc.Offset
	for _, field := range []struct {
		name string
		dest *int
	}{{"gridOffsetX", &offset.X}, {"gridOffsetY", &offset.Y}} {
		raw := strings.TrimSpace(r.FormValue(field.name))
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < -maxGridOffset || n > maxGridOffset {
			writeError(w, http.StatusBadRequest, "grid offset must be a small whole number of pixels")
			return imageproc.Offset{}, false
		}
		*field.dest = n
	}
	return offset, true
}

// listRoomAssets returns the room's library. Authenticated, unlike
// serving an individual asset: a private room's library is a list of
// everything that room has, and handing that out to anyone with the slug
// would leak the contents of a room they can't otherwise see. Serving
// one asset by unguessable ID is a different proposition (see
// serveAsset).
func (srv *Server) listRoomAssets(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	if !srv.requireParticipant(w, r, room) {
		return
	}

	assets, err := srv.store.ListRoomAssets(room.ID)
	if err != nil {
		slog.Error("api: list room assets failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list assets")
		return
	}

	payload := make([]assetPayloadT, 0, len(assets))
	for _, a := range assets {
		payload = append(payload, libraryAssetPayload(a))
	}
	writeJSON(w, http.StatusOK, payload)
}

// checkSession answers whether a session token is still good for this
// room, and nothing else.
//
// It exists for the client's reconnect loop. A failed WebSocket
// handshake reaches the browser as a bare `onclose` with no status, so
// the socket alone cannot tell "the server is restarting, keep trying"
// from "this session is gone, send them back to the join form" — and
// guessing wrong either bounces someone out over a five-second blip or
// retries forever against a room that no longer knows them. This gives
// the same three answers the upgrade does: 200, 401 for a token that
// isn't good, 404 for a room that isn't there.
//
// Deliberately does not echo the token back. The caller already has it,
// and a response that repeats a credential is one redirect or one log
// line away from leaking it.
func (srv *Server) checkSession(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing session token")
		return
	}
	participant, err := srv.store.GetParticipantByToken(room.ID, token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"participantId": participant.ID,
		"displayName":   participant.DisplayName,
		"role":          string(participant.Role),
	})
}

// endSession signs one device out, leaving the seat and every other
// device on it alone — which is the whole of what leaving a room does
// now that a seat outlives a browser. Before seats there was nothing
// server-side to end: the session *was* the identity, so leaving could
// only be a browser forgetting its own localStorage.
//
// Idempotent by construction: a token that no longer resolves is a
// device that is already signed out, and a "leave" that fails because
// you already left would be a worse answer than silence.
func (srv *Server) endSession(w http.ResponseWriter, r *http.Request) {
	room, ok := srv.lookupRoom(w, r.PathValue("slug"))
	if !ok {
		return
	}
	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing session token")
		return
	}
	// Scoped through the room like every other token read, so one room's
	// link can't be used to end a session in another.
	if _, err := srv.store.GetParticipantByToken(room.ID, token); err == nil {
		if err := srv.store.DeleteSession(token); err != nil {
			slog.Error("api: end session failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to leave the room")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// requireParticipant checks the bearer session token belongs to this
// room, which is the whole of "are you allowed to see this room's
// things" — there are no accounts, so holding a session for the room is
// membership.
func (srv *Server) requireParticipant(w http.ResponseWriter, r *http.Request, room store.Room) bool {
	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing session token")
		return false
	}
	if _, err := srv.store.GetParticipantByToken(room.ID, token); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
		return false
	}
	return true
}

// storeImage commits a re-encoded image, reusing an existing asset when
// the same bytes are already held.
//
// The hash is of the re-encoded output rather than of what was uploaded,
// which is what makes dedup meaningful: two people uploading the same
// map as a PNG and as a JPEG are not byte-identical on the way in, but
// the stored WebP is what everyone actually gets, and that's what should
// be shared. It also means the hash always describes a file we can
// serve.
func (srv *Server) storeImage(image imageproc.Result, filename string) (store.Asset, error) {
	hash := sha256.Sum256(image.Data)
	contentHash := hex.EncodeToString(hash[:])

	if existing, err := srv.store.FindAssetByHash(contentHash); err == nil {
		return existing, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return store.Asset{}, err
	}

	if err := srv.blobs.Write(contentHash, bytes.NewReader(image.Data)); err != nil {
		return store.Asset{}, err
	}

	return srv.store.CreateAsset(
		contentHash,
		imageproc.WebPFilename(filename),
		imageproc.MimeType,
		int64(len(image.Data)),
	)
}

// serveAsset streams an asset's bytes back out. It's intentionally
// unauthenticated: asset IDs are unguessable UUIDs, and map/token
// images need to load as plain <img>/canvas sources, which can't
// attach a bearer token.
func (srv *Server) serveAsset(w http.ResponseWriter, r *http.Request) {
	asset, err := srv.store.GetAsset(r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("api: lookup asset failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to look up asset")
		return
	}

	f, err := srv.blobs.Open(asset.ContentHash)
	if err != nil {
		slog.Error("api: open asset blob failed", "error", err)
		writeError(w, http.StatusInternalServerError, "asset file missing")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", asset.MimeType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	io.Copy(w, f)
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}
	return strings.TrimPrefix(auth, prefix)
}
