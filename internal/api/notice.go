package api

import "net/http"

// getNotice answers with the Host's banner message, or an empty string
// when the server was started without one.
//
// Deliberately not a 404 when unset: the client asks once on every page
// load and an empty answer is the ordinary case, not a miss. A status
// code the browser logs as an error for the normal state would have
// every Host reporting a bug.
func (s *Server) getNotice(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"notice": s.notice})
}
