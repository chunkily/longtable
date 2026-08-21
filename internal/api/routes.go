// Package api wires together the HTTP routes: the embedded frontend, the
// REST endpoints, and the WebSocket sync endpoint.
package api

import (
	"io/fs"
	"net/http"
	"strings"

	"longtable/internal/blobstore"
	"longtable/internal/store"
	"longtable/internal/ws"
)

type Server struct {
	store    *store.Store
	hub      *ws.Hub
	blobs    *blobstore.Store
	frontend fs.FS
}

// NewRouter wires the whole HTTP surface.
func NewRouter(
	s *store.Store, hub *ws.Hub, blobs *blobstore.Store, frontend fs.FS,
) http.Handler {
	srv := &Server{store: s, hub: hub, blobs: blobs, frontend: frontend}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// The Host's banner, for everyone on this server whether or not they
	// are in a room — so it answers without a session, like the seat list
	// does. It carries only what the Host typed and nothing about the
	// server, which is what keeps an unauthenticated endpoint dull.
	//
	// Set and cleared with `longtable set-banner "…"` / `clear-banner`
	// rather than from the web UI, on purpose: a Host runs the server and
	// needn't be at any table on it, so there is no screen of theirs to
	// put this on. See planning/roles.md. Those commands reach into the
	// same database this server has open and change nothing here — this
	// handler reads the current value fresh every time, so nothing has to
	// notice a change or restart for it to take effect.
	mux.HandleFunc("GET /api/notice", srv.getNotice)

	// Deliberately no `GET /api/rooms`. There used to be one, and the home
	// page listed every room on the server to anyone who loaded it — which
	// meant the names of everyone's games were readable by anyone who
	// could reach the machine. Rooms are reached by being given their
	// link; a browser's own list comes from its localStorage sessions.
	// The Host's `longtable room list` still enumerates them, which is the
	// right place for it since it needs the database file.
	mux.HandleFunc("POST /api/rooms", srv.createRoom)
	mux.HandleFunc("POST /api/rooms/{slug}/join", srv.joinRoom)
	mux.HandleFunc("POST /api/rooms/{slug}/gm-login", srv.gmLogin)
	mux.HandleFunc("GET /api/rooms/{slug}/session", srv.checkSession)
	mux.HandleFunc("DELETE /api/rooms/{slug}/session", srv.endSession)
	// The one endpoint that answers before the caller has a session,
	// because it is what a device with no session looks at. Scoped to a
	// room whose link the caller already holds, and thin on purpose —
	// see listSeats.
	mux.HandleFunc("GET /api/rooms/{slug}/seats", srv.listSeats)
	// Checked before a Player has picked a seat or typed a name, so a
	// wrong password is refused immediately rather than after the rest of
	// the form. Unauthenticated for the same reason listSeats is.
	mux.HandleFunc("POST /api/rooms/{slug}/join-password/check", srv.checkJoinPassword)
	mux.HandleFunc("POST /api/rooms/{slug}/seats", srv.createSeat)
	mux.HandleFunc("DELETE /api/rooms/{slug}/seats/{id}", srv.deleteSeat)
	// Changing the room's own password, for a GM who is signed in. Named
	// to match the gm-login it governs, and a PUT because it replaces the
	// one value rather than adding anything.
	mux.HandleFunc("PUT /api/rooms/{slug}/gm-password", srv.setGMPassword)
	// Separate from the GM password above: this one gates joining as a
	// Player rather than the GM seat. A PUT for the same reason — it
	// replaces the one value (or clears it) rather than adding anything.
	mux.HandleFunc("PUT /api/rooms/{slug}/join-password", srv.setJoinPassword)
	// The one endpoint that ends a room. GM-only, and the only thing in
	// the app with no undo behind it.
	mux.HandleFunc("DELETE /api/rooms/{slug}", srv.deleteRoom)
	mux.HandleFunc("GET /api/rooms/{slug}/assets", srv.listRoomAssets)
	mux.HandleFunc("POST /api/rooms/{slug}/assets", srv.uploadAsset)
	mux.HandleFunc("PATCH /api/rooms/{slug}/assets/{id}", srv.updateRoomAsset)
	mux.HandleFunc("DELETE /api/rooms/{slug}/assets/{id}", srv.removeRoomAsset)
	mux.HandleFunc("GET /api/assets/{id}", srv.serveAsset)

	mux.HandleFunc("GET /ws", hub.ServeHTTP)

	mux.Handle("/", spaHandler(frontend))

	return mux
}

// spaHandler serves the embedded static build, falling back to
// index.html for any path that isn't a real file — this mirrors the
// adapter-static `fallback: 'index.html'` SPA config on the frontend, so
// client-side routes (e.g. /game/:id) resolve correctly on a hard
// refresh.
//
// The fallback is served by hand rather than by rewriting r.URL.Path and
// delegating to http.FileServerFS: that handler treats any path ending
// in "index.html" as a canonicalization case and 301-redirects to "/",
// which would silently drop the deep-linked route.
func spaHandler(frontend fs.FS) http.Handler {
	fileServer := http.FileServerFS(frontend)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		if _, err := fs.Stat(frontend, name); err != nil {
			serveIndex(w, frontend)
			return
		}

		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, frontend fs.FS) {
	data, err := fs.ReadFile(frontend, "index.html")
	if err != nil {
		http.Error(w, "index.html not found in embedded frontend", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
