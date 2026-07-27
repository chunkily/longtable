package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"longtable/internal/store"
)

const maxAssetUploadBytes = 25 << 20 // 25 MiB — generous for map/token art

type assetPayloadT struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	ByteSize int64  `json:"byteSize"`
}

func assetPayload(a store.Asset) assetPayloadT {
	return assetPayloadT{ID: a.ID, Filename: a.Filename, MimeType: a.MimeType, ByteSize: a.ByteSize}
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

	token := bearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing session token")
		return
	}
	if _, err := srv.store.GetParticipantByToken(room.ID, token); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
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

	asset, err := srv.storeUpload(file, header.Filename, header.Header.Get("Content-Type"))
	if err != nil {
		slog.Error("api: upload asset failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to store upload")
		return
	}

	writeJSON(w, http.StatusCreated, assetPayload(asset))
}

// storeUpload hashes src while streaming it to a temp file, then either
// reuses an existing asset with the same content hash or commits the
// temp file into the blob store and records a new asset row.
func (srv *Server) storeUpload(src io.Reader, filename, mimeType string) (store.Asset, error) {
	tmp, err := os.CreateTemp("", "longtable-upload-*")
	if err != nil {
		return store.Asset{}, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(hasher, tmp), src)
	if err != nil {
		return store.Asset{}, err
	}
	hash := hex.EncodeToString(hasher.Sum(nil))

	if existing, err := srv.store.FindAssetByHash(hash); err == nil {
		return existing, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return store.Asset{}, err
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return store.Asset{}, err
	}
	if err := srv.blobs.Write(hash, tmp); err != nil {
		return store.Asset{}, err
	}

	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return srv.store.CreateAsset(hash, filename, mimeType, size)
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
