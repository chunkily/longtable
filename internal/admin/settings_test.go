package admin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"longtable/internal/config"
)

// signedIn is a handler plus the cookie that gets past the guard, so
// these tests are about the settings page rather than about auth.
func signedIn(t *testing.T, settings Settings) (http.Handler, *http.Cookie) {
	t.Helper()

	handler := New(fixed(token), adminAddr, "http://127.0.0.1:8080", settings)
	resp := get(t, handler, "/?token="+token, adminAddr)
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			return handler, c
		}
	}
	t.Fatal("could not sign in")
	return nil, nil
}

func post(t *testing.T, handler http.Handler, form url.Values, session *http.Cookie) *http.Response {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Host = adminAddr
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(session)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Result()
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(data)
}

// The page is the only place a Host is told what a setting does, now
// that the commented file is gone — so a setting rendered without its
// summary is a setting nobody can act on.
func TestSettingsPage_ShowsEverySettingWithItsMeaning(t *testing.T) {
	handler, session := signedIn(t, &fakeSettings{values: map[string]string{"addr": ":9000"}})

	page := body(t, get(t, handler, "/", adminAddr, session))

	for _, setting := range config.Registry {
		if !strings.Contains(page, setting.Key) {
			t.Errorf("%s isn't on the page", setting.Key)
		}
		opening := strings.Join(strings.Fields(setting.Summary)[:4], " ")
		if !strings.Contains(page, opening) {
			t.Errorf("%s is shown with no explanation", setting.Key)
		}
		if !strings.Contains(page, setting.Default) {
			t.Errorf("%s doesn't show its default", setting.Key)
		}
	}

	// The stored value, not the default, is what's in the box.
	if !strings.Contains(page, `value=":9000"`) {
		t.Error("the stored addr isn't in the form")
	}
	// Nothing is live yet, so the page has to say so rather than let a
	// save imply a change that hasn't happened.
	if !strings.Contains(page, "needs a restart") {
		t.Error("nothing says a change needs a restart")
	}
	// And a Host who came here looking for their rooms is pointed on.
	if !strings.Contains(page, "http://127.0.0.1:8080") {
		t.Error("the page doesn't say where Longtable itself is")
	}
}

func TestSettingsPage_SavesWhatChanged(t *testing.T) {
	store := &fakeSettings{}
	handler, session := signedIn(t, store)

	form := url.Values{}
	for _, setting := range config.Registry {
		form.Set(setting.Key, setting.Default)
	}
	form.Set("departure_grace", "2m")

	resp := post(t, handler, form, session)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want a redirect after saving", resp.StatusCode)
	}
	// Redirect after posting, so a refresh doesn't save again.
	if location := resp.Header.Get("Location"); !strings.Contains(location, "saved") {
		t.Errorf("redirected to %q, which won't confirm anything", location)
	}

	if store.values["departure_grace"] != "2m0s" {
		t.Errorf("departure_grace = %q, want the canonical 2m0s", store.values["departure_grace"])
	}
	// Only the edited field was written. A save that stored every field
	// would lose the distinction `config list` reports between a setting
	// somebody chose and one still on its default.
	if len(store.values) != 1 {
		t.Errorf("saved %d settings, want only the one that changed: %v", len(store.values), store.values)
	}
}

// The whole reason validation is here rather than at the next startup:
// the person can still see what they typed.
func TestSettingsPage_ShowsAMistakeBesideTheFieldAndKeepsWhatWasTyped(t *testing.T) {
	store := &fakeSettings{}
	handler, session := signedIn(t, store)

	form := url.Values{}
	for _, setting := range config.Registry {
		form.Set(setting.Key, setting.Default)
	}
	form.Set("departure_grace", "soon")

	resp := post(t, handler, form, session)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want the form back", resp.StatusCode)
	}

	page := body(t, resp)
	if !strings.Contains(page, `value="soon"`) {
		t.Error("what was typed was thrown away, so the complaint is about something invisible")
	}
	if !strings.Contains(page, "isn&#39;t a length of time") && !strings.Contains(page, "isn't a length of time") {
		t.Errorf("the reason isn't on the page:\n%s", page)
	}
	if !strings.Contains(page, `aria-invalid="true"`) {
		t.Error("the bad field isn't marked invalid, so a screen reader gets nothing")
	}
	if len(store.values) != 0 {
		t.Errorf("something was saved despite the error: %v", store.values)
	}
}

// A rule about two settings at once has no field to sit beside, and
// neither value is wrong on its own.
func TestSettingsPage_ReportsACrossSettingProblem(t *testing.T) {
	store := &fakeSettings{}
	handler, session := signedIn(t, store)

	form := url.Values{}
	for _, setting := range config.Registry {
		form.Set(setting.Key, setting.Default)
	}
	form.Set("addr", ":8081")
	form.Set("admin_addr", "127.0.0.1:8081")

	page := body(t, post(t, handler, form, session))
	if !strings.Contains(page, "both on port 8081") {
		t.Errorf("the port clash isn't reported:\n%s", page)
	}
	if len(store.values) != 0 {
		t.Errorf("a clashing pair was saved: %v", store.values)
	}
}

// Nothing is written until the whole form is known to be good — a
// half-applied save leaves a server in a state nobody asked for and no
// screen describing.
func TestSettingsPage_SavesNothingWhenOneFieldIsBad(t *testing.T) {
	store := &fakeSettings{}
	handler, session := signedIn(t, store)

	form := url.Values{}
	for _, setting := range config.Registry {
		form.Set(setting.Key, setting.Default)
	}
	form.Set("addr", ":9000")                      // fine
	form.Set("uploads.max_asset_size", "enormous") // not

	post(t, handler, form, session)
	if len(store.values) != 0 {
		t.Errorf("the good field was written even though the form failed: %v", store.values)
	}
}

// admin_addr can be edited from the page it serves, which is allowed —
// the CLI can always put it back — but the warning has to be visible
// before somebody does it, not after.
func TestSettingsPage_WarnsAboutExposingItself(t *testing.T) {
	handler, session := signedIn(t, &fakeSettings{values: map[string]string{"admin_addr": ":8081"}})

	page := body(t, get(t, handler, "/", adminAddr, session))
	if !strings.Contains(page, "Anyone on your network") {
		t.Errorf("an admin page open to the network says nothing about it:\n%s", page)
	}
}

// The form is behind the same guard as everything else — a POST is the
// half it would be easiest to forget.
func TestSettingsPage_SavingNeedsTheSameAuth(t *testing.T) {
	store := &fakeSettings{}
	handler := New(fixed(token), adminAddr, "http://127.0.0.1:8080", store)

	form := url.Values{"addr": {":9999"}}
	resp := post(t, handler, form, &http.Cookie{Name: cookieName, Value: "forged"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if len(store.values) != 0 {
		t.Errorf("an unauthenticated post changed settings: %v", store.values)
	}
}
