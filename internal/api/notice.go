package api

import (
	"log/slog"
	"net/http"
)

// getNotice answers with the Host's current banner message, or an empty
// string when none is set.
//
// Read fresh from the store on every call rather than held on Server:
// the whole point of `longtable set-banner`/`clear-banner` is that they
// change what a *running* server answers, so nothing here may cache a
// value across requests.
//
// Deliberately not a 404 when unset: the client asks once on every page
// load and an empty answer is the ordinary case, not a miss. A status
// code the browser logs as an error for the normal state would have
// every Host reporting a bug.
func (s *Server) getNotice(w http.ResponseWriter, r *http.Request) {
	notice, err := s.store.GetBanner()
	if err != nil {
		slog.Error("api: get banner failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load the banner")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"notice": notice})
}
