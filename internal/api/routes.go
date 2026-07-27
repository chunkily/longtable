// Package api wires together the HTTP routes: the embedded frontend, the
// REST endpoints, and the WebSocket sync endpoint.
package api

import (
	"database/sql"
	"io/fs"
	"net/http"
	"strings"

	"longtable/internal/ws"
)

// NewRouter builds the top-level HTTP handler. database is threaded
// through for future REST handlers (none exist yet beyond the health
// check).
func NewRouter(hub *ws.Hub, database *sql.DB, frontend fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

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
