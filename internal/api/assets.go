package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
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

type assetPayloadT struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	ByteSize int64  `json:"byteSize"`
	// Attribution is whatever free text the uploader gave as credit or
	// licence for this room's copy of the asset. Empty when none was
	// given, which is the common case.
	Attribution string `json:"attribution"`
	// Flattened says an animated upload was accepted as a still image, so
	// the uploader can be told rather than left wondering why their
	// goblin stopped moving. Absent for the ordinary case.
	Flattened bool `json:"flattened,omitempty"`
}

func assetPayload(a store.Asset) assetPayloadT {
	return assetPayloadT{ID: a.ID, Filename: a.Filename, MimeType: a.MimeType, ByteSize: a.ByteSize}
}

func libraryAssetPayload(a store.LibraryAsset) assetPayloadT {
	payload := assetPayload(a.Asset)
	payload.Attribution = a.Attribution
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

	// Re-encoded before anything else looks at it: from here on the only
	// bytes in play are ones this program produced from the decoded
	// pixels. See internal/imageproc for why that matters.
	image, err := imageproc.Reencode(file)
	if err != nil {
		switch {
		case errors.Is(err, imageproc.ErrUnsupportedFormat):
			writeError(w, http.StatusBadRequest,
				"that file isn't a PNG, JPEG, WebP or GIF image")
		case errors.Is(err, imageproc.ErrTooLarge):
			writeError(w, http.StatusBadRequest, "image dimensions are too large")
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
	attribution := strings.TrimSpace(r.FormValue("attribution"))
	if len(attribution) > maxAttributionLength {
		attribution = attribution[:maxAttributionLength]
	}
	if err := srv.store.AddAssetToRoom(room.ID, asset.ID, attribution); err != nil {
		slog.Error("api: add asset to room library failed", "error", err)
		writeError(w, http.StatusInternalServerError, "stored the image, but failed to add it to the library")
		return
	}

	payload := assetPayload(asset)
	payload.Attribution = attribution
	payload.Flattened = image.Animated
	writeJSON(w, http.StatusCreated, payload)
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
