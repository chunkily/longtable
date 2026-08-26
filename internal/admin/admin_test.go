package admin

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	token     = "a-long-random-looking-token"
	adminAddr = "127.0.0.1:8081"
)

// fakeSettings stands in for the store, so the admin surface can be
// tested without a database.
type fakeSettings struct {
	values map[string]string
	failOn string
}

func (f *fakeSettings) Settings() (map[string]string, error) {
	if f.failOn == "read" {
		return nil, errors.New("database is gone")
	}
	out := map[string]string{}
	for k, v := range f.values {
		out[k] = v
	}
	return out, nil
}

func (f *fakeSettings) SetSetting(key, value string) error {
	if f.failOn == "write" {
		return errors.New("disk is full")
	}
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[key] = value
	return nil
}

// fixed is a token that never changes, for the tests that aren't about
// rotation.
func fixed(token string) func() (string, error) {
	return func() (string, error) { return token, nil }
}

// get issues a request the way a browser on the admin port would, and
// does not follow redirects — the redirect is the thing under test in
// half of these.
func get(t *testing.T, handler http.Handler, target, host string, cookies ...*http.Cookie) *http.Response {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = host
	for _, c := range cookies {
		req.AddCookie(c)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Result()
}

func TestGuard_TokenGetsYouIn(t *testing.T) {
	handler := New(fixed(token), adminAddr, "http://127.0.0.1:8080", &fakeSettings{}, nil)

	resp := get(t, handler, "/?token="+token, adminAddr)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want a redirect", resp.StatusCode)
	}

	// The token must not survive into the address bar, the history or a
	// referrer, which is the correction Jupyter made after shipping the
	// naive version.
	location := resp.Header.Get("Location")
	if strings.Contains(location, "token") {
		t.Errorf("redirected to %q, which still carries the token", location)
	}

	var session *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			session = c
		}
	}
	if session == nil {
		t.Fatal("no session cookie was set, so the redirect lands right back on the door")
	}
	// SameSite=Strict is what stops a page the Host merely visited from
	// having the browser attach this to a request of its own. Blind CSRF
	// against a loopback port is a real technique and an auto-sent cookie
	// is precisely what it needs.
	if session.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie SameSite = %v, want Strict", session.SameSite)
	}
	if !session.HttpOnly {
		t.Error("cookie is readable from JavaScript")
	}

	// And the cookie alone gets in from then on.
	page := get(t, handler, "/", adminAddr, session)
	if page.StatusCode != http.StatusOK {
		t.Fatalf("status with the session cookie = %d, want 200", page.StatusCode)
	}
}

func TestGuard_RefusesEverythingElse(t *testing.T) {
	handler := New(fixed(token), adminAddr, "http://127.0.0.1:8080", &fakeSettings{}, nil)

	for name, request := range map[string]func() *http.Response{
		"no token at all": func() *http.Response {
			return get(t, handler, "/", adminAddr)
		},
		"a wrong token": func() *http.Response {
			return get(t, handler, "/?token=not-the-token", adminAddr)
		},
		"a token that is a prefix of the real one": func() *http.Response {
			return get(t, handler, "/?token="+token[:10], adminAddr)
		},
		"a forged cookie": func() *http.Response {
			return get(t, handler, "/", adminAddr, &http.Cookie{Name: cookieName, Value: "nope"})
		},
	} {
		if status := request().StatusCode; status != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, status)
		}
	}
}

// The DNS-rebinding defence. A page the Host is merely visiting can
// point a name it controls at 127.0.0.1 and talk to this server, and the
// one thing it cannot do is forge the Host header the browser sends.
func TestGuard_RefusesAHostHeaderItDoesNotServe(t *testing.T) {
	handler := New(fixed(token), adminAddr, "http://127.0.0.1:8080", &fakeSettings{}, nil)

	for _, host := range []string{
		"evil.example.com:8081",
		"longtable.attacker.test:8081",
		// Right name, wrong port: not this listener.
		"127.0.0.1:9999",
		"",
	} {
		resp := get(t, handler, "/?token="+token, host)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Host %q: status = %d, want 404 — a valid token must not be enough", host, resp.StatusCode)
		}
	}
}

// A Host reaches their own machine by several names, and all of them are
// the same machine.
func TestGuard_AcceptsEveryNameForThisMachine(t *testing.T) {
	handler := New(fixed(token), adminAddr, "http://127.0.0.1:8080", &fakeSettings{}, nil)

	for _, host := range []string{"127.0.0.1:8081", "localhost:8081", "[::1]:8081"} {
		resp := get(t, handler, "/?token="+token, host)
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("Host %q: status = %d, want the redirect that means it was accepted", host, resp.StatusCode)
		}
	}
}

// A Host who moved the listener off loopback deliberately gets what they
// asked for: their own name works, and the loopback shorthand no longer
// does, because it isn't what this is served on any more.
func TestGuard_MatchesTheConfiguredHostWhenItIsNotLoopback(t *testing.T) {
	handler := New(fixed(token), "192.168.1.23:8081", "http://127.0.0.1:8080", &fakeSettings{}, nil)

	if resp := get(t, handler, "/?token="+token, "192.168.1.23:8081"); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("the configured address was refused: %d", resp.StatusCode)
	}
	if resp := get(t, handler, "/?token="+token, "evil.example.com:8081"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("a foreign Host header was accepted: %d", resp.StatusCode)
	}
}

// A server whose token has somehow not been minted must not be an open
// door — an empty stored token would otherwise match an empty cookie.
func TestGuard_AnEmptyTokenLetsNobodyIn(t *testing.T) {
	handler := New(fixed(""), adminAddr, "http://127.0.0.1:8080", &fakeSettings{}, nil)

	if resp := get(t, handler, "/", adminAddr, &http.Cookie{Name: cookieName, Value: ""}); resp.StatusCode == http.StatusOK {
		t.Fatal("an empty token was accepted")
	}
	if resp := get(t, handler, "/?token=", adminAddr); resp.StatusCode == http.StatusOK {
		t.Fatal("an empty token in the query was accepted")
	}
}

// The bug this was written after: the running server used to hold the
// token from startup, so `longtable admin -new` printed "any link you
// shared before now stops working" while the old link kept working and
// the new one was refused until a restart — exactly backwards for a Host
// rotating because they leaked it.
func TestGuard_RotationTakesEffectImmediately(t *testing.T) {
	current := "first-token"
	handler := New(func() (string, error) { return current, nil }, adminAddr, "http://127.0.0.1:8080", &fakeSettings{}, nil)

	// A Host is in, holding a cookie.
	resp := get(t, handler, "/?token="+current, adminAddr)
	var session *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			session = c
		}
	}
	if session == nil {
		t.Fatal("no session cookie")
	}
	if page := get(t, handler, "/", adminAddr, session); page.StatusCode != http.StatusOK {
		t.Fatalf("status before rotating = %d, want 200", page.StatusCode)
	}

	current = "second-token"

	// The cookie they were holding stops working, with no restart.
	if page := get(t, handler, "/", adminAddr, session); page.StatusCode != http.StatusUnauthorized {
		t.Errorf("the old cookie still works after rotating: %d", page.StatusCode)
	}
	// And so does the old link.
	if page := get(t, handler, "/?token=first-token", adminAddr); page.StatusCode != http.StatusUnauthorized {
		t.Errorf("the old link still works after rotating: %d", page.StatusCode)
	}
	// The new one gets in.
	if page := get(t, handler, "/?token=second-token", adminAddr); page.StatusCode != http.StatusSeeOther {
		t.Errorf("the new link was refused: %d", page.StatusCode)
	}
}

// A database that cannot be read is not a reason to let someone in.
func TestGuard_ARefusedReadIsNotAPass(t *testing.T) {
	handler := New(func() (string, error) { return "", errors.New("database is gone") }, adminAddr, "http://127.0.0.1:8080", &fakeSettings{}, nil)

	if resp := get(t, handler, "/?token=anything", adminAddr); resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}
