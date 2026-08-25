// Package admin is the Host's own HTTP surface: the settings screen and
// whatever else only the person running the server should reach.
//
// It is a **separate listener** from the game, bound to loopback by
// default, and that is the load-bearing part of its security rather than
// the token ([ADR-0009]). The obvious alternative — one listener, with
// these routes refusing a non-loopback client — fails in a way nobody
// would notice: put nginx or Caddy in front and every request arrives
// from 127.0.0.1, the check passes for the entire internet, and nothing
// looks wrong. A socket actually bound to 127.0.0.1 cannot be reached
// from off the machine whatever headers arrive.
//
// [ADR-0007] trusts the table; this is deliberately outside it. Role
// boundaries inside a room are one thing, and a server's configuration
// is not a room matter — a Host runs the server and need not be at any
// table (planning/roles.md).
//
// [ADR-0009]: ../../planning/decisions/0009-admin-ui-and-database-config.md
// [ADR-0007]: ../../planning/decisions/0007-the-table-is-trusted.md
package admin

import (
	"crypto/subtle"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// cookieName holds the session a token is exchanged for.
const cookieName = "longtable_admin"

// Server is the admin surface.
//
// The token is read on every request rather than held from startup, and
// that is not a performance oversight — it is what makes rotating one
// mean something. A cached token leaves `longtable admin -new` claiming
// "any link you shared before now stops working" while the old link
// keeps working and the *new* one is refused, which is precisely
// backwards for a Host rotating because they leaked it. The banner does
// the same thing for the same reason; this surface is one person
// clicking, so a read per request costs nothing worth having.
type Server struct {
	token     func() (string, error)
	adminAddr string
	// gameURL is Longtable itself, so this page can point back at it.
	gameURL string
}

// New wires the admin routes. adminAddr is the address this will be
// served on, kept so the Host header can be checked against it; gameURL
// is where Longtable itself is, so a Host who arrived here looking for
// their rooms is sent on rather than left.
func New(token func() (string, error), adminAddr, gameURL string) http.Handler {
	srv := &Server{token: token, adminAddr: adminAddr, gameURL: gameURL}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", srv.index)

	return srv.guard(mux)
}

// guard is the whole of the authentication.
//
// Three things happen here, and each covers something the others don't:
//
//   - The Host header is checked, which is the DNS-rebinding defence. A
//     page the Host is merely *visiting* can point a name it controls at
//     127.0.0.1 and talk to this server as though it were same-origin;
//     what it cannot do is forge the Host header the browser sends.
//     Jupyter shipped without this and had to add it.
//   - A token in the query string is exchanged for a cookie and then
//     redirected away, so the secret does not sit in the address bar,
//     the browser's history, or a referrer.
//   - The cookie is SameSite=Strict, so a form or fetch from another
//     origin does not carry it. Blind CSRF against a loopback port is a
//     real technique, and an auto-sent cookie is exactly what it needs.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			// Deliberately unhelpful: whoever is on the other end of a
			// mismatched Host header is not a Host who has mistyped
			// something, and telling them what was expected helps only them.
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if offered := r.URL.Query().Get("token"); offered != "" {
			current, err := s.tokenMatches(offered)
			if err != nil {
				slog.Error("admin: read token failed", "error", err)
				http.Error(w, "something went wrong", http.StatusInternalServerError)
				return
			}
			if !current {
				http.Error(w, "that link has expired. Run \"longtable admin\" for a new one", http.StatusUnauthorized)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     cookieName,
				Value:    offered,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
				// No Secure: this is plain HTTP with no TLS, and a Secure
				// cookie would simply never be sent. The limitation is real
				// and documented rather than papered over — see docs/hosting.md.
			})
			// Redirect rather than serve, so the token leaves the URL.
			stripped := *r.URL
			query := stripped.Query()
			query.Del("token")
			stripped.RawQuery = query.Encode()
			http.Redirect(w, r, stripped.RequestURI(), http.StatusSeeOther)
			return
		}

		cookie, err := r.Cookie(cookieName)
		if err != nil {
			http.Error(w, "run \"longtable admin\" to get in", http.StatusUnauthorized)
			return
		}
		ok, err := s.tokenMatches(cookie.Value)
		if err != nil {
			slog.Error("admin: read token failed", "error", err)
			http.Error(w, "something went wrong", http.StatusInternalServerError)
			return
		}
		if !ok {
			// Covers a rotated token as well as a forged one: the cookie a
			// Host is holding stops working the moment a new one is issued,
			// which is the whole point of issuing one.
			http.Error(w, "run \"longtable admin\" to get in", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// tokenMatches compares in constant time. The lengths are equal and the
// values are random, so a timing signal here is not much of a foothold —
// but it costs one function call to not have to argue about it.
//
// An empty stored token matches nothing, so a server whose token somehow
// never got minted is a closed door rather than an open one.
func (s *Server) tokenMatches(candidate string) (bool, error) {
	current, err := s.token()
	if err != nil {
		return false, err
	}
	if current == "" {
		return false, nil
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(current)) == 1, nil
}

// hostAllowed checks the Host header names the address this is actually
// served on.
//
// Compared by port, with the host part only having to *be* a loopback
// name rather than match exactly: a Host may reach this as `localhost`,
// `127.0.0.1` or `[::1]`, and all three are the same machine. An
// attacker's domain resolving to 127.0.0.1 is none of them.
//
// When the admin listener has been moved off loopback deliberately, this
// falls back to matching the configured host exactly — a Host who did
// that gets what they asked for.
func (s *Server) hostAllowed(host string) bool {
	if host == "" {
		return false
	}

	wantHost, wantPort, err := net.SplitHostPort(s.adminAddr)
	if err != nil {
		return false
	}

	gotHost, gotPort, err := net.SplitHostPort(host)
	if err != nil {
		// No port in the header: only valid if the listener is on 80.
		gotHost, gotPort = host, "80"
	}
	if gotPort != wantPort {
		return false
	}

	if isLoopbackHost(wantHost) {
		return isLoopbackHost(gotHost)
	}
	return strings.EqualFold(gotHost, wantHost)
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// index is a placeholder until the settings screen lands.
//
// It exists so the surface can be tested as a surface — that the token
// gets someone in, that nothing else does — before there is anything
// worth protecting on it.
//
// It names the game's address because a Host who has just met two
// addresses will sooner or later open the wrong one, and this is the
// wrong one. Landing here with no way over is how "the page I'm seeing
// is not the room list" happens.
func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := placeholder.Execute(w, s.gameURL); err != nil {
		slog.Error("admin: write index failed", "error", err)
	}
}

// html/template rather than a string: the address it interpolates comes
// from a Host's own settings, and a template that knows it is writing an
// href escapes it for that context.
var placeholder = template.Must(template.New("admin").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Longtable admin</title>
<style>
  body { font-family: system-ui, sans-serif; margin: 4rem auto; max-width: 32rem; line-height: 1.5; padding: 0 1rem; }
  .rooms { padding: 1rem; border: 1px solid #ccc; border-radius: 6px; }
</style>
<h1>Longtable admin</h1>
<p>This is the admin page. It's for settings, and only this machine can reach it.</p>
<p class="rooms"><strong>Looking for your rooms?</strong> Longtable is at
  <a href="{{.}}">{{.}}</a> — that's the address to share with players.</p>
<p>There's nothing to change here yet; the settings screen is still being built.
  In the meantime, <code>longtable config list</code> shows every setting and what it does.</p>
`))
