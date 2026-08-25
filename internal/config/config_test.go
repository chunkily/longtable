package config

import (
	"strings"
	"testing"
	"time"

	"github.com/dustin/go-humanize"

	"longtable/internal/ws"
)

// The registry is the only description of a setting there is now that
// the config file is gone, so a row that can't describe itself is a
// setting a Host meets with no explanation at all.
func TestRegistry_EverySettingDescribesItself(t *testing.T) {
	for _, s := range Registry {
		if s.Key == "" {
			t.Fatal("a setting has no key")
		}
		if s.Summary == "" {
			t.Errorf("%s has no summary, and the summary is what the config file's comment used to be", s.Key)
		}
		if s.Parse == nil || s.apply == nil {
			t.Errorf("%s can't be parsed or applied", s.Key)
		}
		if s.Default == "" {
			t.Errorf("%s has no default", s.Key)
		}
	}
}

// Defaults() panics if this is ever false, so it is worth failing here
// first, where the message says which setting.
func TestRegistry_DefaultsAreValid(t *testing.T) {
	for _, s := range Registry {
		canonical, err := s.Parse(s.Default)
		if err != nil {
			t.Errorf("%s: default %q doesn't pass its own Parse: %v", s.Key, s.Default, err)
			continue
		}
		// A default that isn't already canonical would print back
		// differently from how it was written, which is the sort of thing
		// that makes a settings screen look broken.
		if canonical != s.Default {
			t.Errorf("%s: default %q canonicalises to %q", s.Key, s.Default, canonical)
		}
	}
	if err := Defaults().Validate(); err != nil {
		t.Errorf("the defaults don't satisfy their own cross-setting rules: %v", err)
	}
}

// Keys are typed by people and stored as text; a dot means something to
// the export's sectioning, and spaces or capitals would make a key that
// only sometimes matches what was typed.
func TestRegistry_KeysAreTypeable(t *testing.T) {
	seen := map[string]bool{}
	for _, key := range Keys() {
		if seen[key] {
			t.Errorf("%s appears twice", key)
		}
		seen[key] = true
		if key != strings.ToLower(key) || strings.ContainsAny(key, " \t") {
			t.Errorf("%q should be lowercase with no spaces", key)
		}
	}
}

// What `set` accepts is what `get` prints back. A Host who types a plain
// byte count should not have to wonder whether it took.
func TestParse_Canonicalises(t *testing.T) {
	for _, tc := range []struct{ key, in, want string }{
		{"uploads.max_asset_size", "26214400", "25 MiB"},
		{"uploads.max_asset_size", "25MiB", "25 MiB"},
		{"uploads.max_total_storage", "2 GB", "1.9 GiB"},
		{"departure_grace", "120s", "2m0s"},
		{"departure_grace", "  2m  ", "2m0s"},
		{"addr", " :9000 ", ":9000"},
	} {
		s, ok := Lookup(tc.key)
		if !ok {
			t.Fatalf("no setting %q", tc.key)
		}
		got, err := s.Parse(tc.in)
		if err != nil {
			t.Errorf("%s: Parse(%q): %v", tc.key, tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: Parse(%q) = %q, want %q", tc.key, tc.in, got, tc.want)
		}
	}
}

// Refused on the way in, where the person who typed it is still looking
// at it. With a file the same mistakes surfaced as a server that
// wouldn't start.
func TestParse_RefusesWhatCannotBeRun(t *testing.T) {
	for name, tc := range map[string]struct{ key, value string }{
		"a duration that is not one": {"departure_grace", "soon"},
		"a negative duration":        {"departure_grace", "-5s"},
		"a zero duration":            {"departure_grace", "0s"},
		"a size that is not one":     {"uploads.max_asset_size", "huge"},
		"a zero size":                {"uploads.max_asset_size", "0"},
		"an empty address":           {"addr", ""},
		"an empty assets directory":  {"assets", "   "},
	} {
		s, ok := Lookup(tc.key)
		if !ok {
			t.Fatalf("no setting %q", tc.key)
		}
		if _, err := s.Parse(tc.value); err == nil {
			t.Errorf("%s: %s accepted %q", name, tc.key, tc.value)
		}
	}
}

// The one rule no single setting can judge alone.
func TestValidate_CatchesACeilingBelowItsFloor(t *testing.T) {
	c, err := Resolve(map[string]string{
		"uploads.max_asset_size":    "1 GiB",
		"uploads.max_total_storage": "1 MiB",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := c.Validate(); err == nil {
		t.Fatal("a total storage limit below the per-asset limit was accepted")
	}
}

// Turning the admin page off is an empty admin_addr rather than a
// separate toggle, so that a toggle and an address can never disagree
// about whether it is on.
func TestAdminAddr_EmptyTurnsItOff(t *testing.T) {
	setting, ok := Lookup("admin_addr")
	if !ok {
		t.Fatal("no admin_addr setting")
	}
	if _, err := setting.Parse(""); err != nil {
		t.Fatalf("an empty admin_addr was refused: %v", err)
	}

	c, err := Resolve(map[string]string{"admin_addr": ""})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// The bug this guards: Resolve used to treat a stored empty string as
	// an absent one and hand back the default, so turning the admin page
	// off would have silently left it on — an edit that does nothing,
	// which is the failure this whole design exists to refuse.
	if c.AdminAddr != "" {
		t.Fatalf("AdminAddr = %q, want empty — the setting was ignored", c.AdminAddr)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("a server with no admin page failed validation: %v", err)
	}

	// And it says nothing about being exposed, because it isn't listening.
	if warning := setting.Warn(""); warning != "" {
		t.Errorf("an admin page that is off warned about the network: %q", warning)
	}
}

// The address every *other* setting refuses to be empty, because a
// server with nowhere to listen is not a server.
func TestAddr_CannotBeTurnedOff(t *testing.T) {
	setting, _ := Lookup("addr")
	if _, err := setting.Parse(""); err == nil {
		t.Fatal("an empty addr was accepted")
	}
}

// Two listeners on one port is never meant, and it fails differently on
// different systems: Linux refuses the second bind, while Windows allows
// a wildcard and a specific address to share a port and gives loopback
// to the more specific one. A Host whose game was on 8081 found the
// admin page on localhost and their rooms still on the LAN address, with
// nothing logged. Refused here so both behave the same.
func TestValidate_RefusesTwoListenersOnOnePort(t *testing.T) {
	for name, values := range map[string]map[string]string{
		"a wildcard against loopback": {"addr": ":8081", "admin_addr": "127.0.0.1:8081"},
		"the same address twice":      {"addr": "127.0.0.1:8080", "admin_addr": "127.0.0.1:8080"},
		"different hosts, one port":   {"addr": "192.168.1.5:9000", "admin_addr": "127.0.0.1:9000"},
	} {
		c, err := Resolve(values)
		if err != nil {
			t.Fatalf("%s: Resolve: %v", name, err)
		}
		if err := c.Validate(); err == nil {
			t.Errorf("%s: accepted %v", name, values)
		}
	}

	// The defaults are two different ports, and must stay that way.
	if err := Defaults().Validate(); err != nil {
		t.Errorf("the defaults collide: %v", err)
	}
}

// Anything the database doesn't mention takes its default, which is what
// lets a later version add a setting without every database already out
// there having to know about it.
func TestResolve_FillsTheGapsWithDefaults(t *testing.T) {
	c, err := Resolve(map[string]string{"departure_grace": "2m"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if time.Duration(c.DepartureGrace) != 2*time.Minute {
		t.Errorf("departure_grace = %s, want 2m", c.DepartureGrace)
	}
	if c.Addr != Defaults().Addr || c.Assets != Defaults().Assets {
		t.Errorf("an unmentioned setting lost its default: %+v", c)
	}
	if c.Uploads.MaxAssetSize != ByteSize(25*humanize.MiByte) {
		t.Errorf("max_asset_size = %s, want 25 MiB", c.Uploads.MaxAssetSize)
	}
}

// A key the registry has never heard of must not stop a server starting.
// Refusing one is the write path's job, where somebody is present to be
// told about it — at startup it would only mean a version that removed a
// setting cannot boot on a database that still remembers it.
func TestResolve_IgnoresAKeyItDoesNotKnow(t *testing.T) {
	c, err := Resolve(map[string]string{"banner_colour": "puce"})
	if err != nil {
		t.Fatalf("an unknown stored key stopped the server: %v", err)
	}
	if c.Addr != Defaults().Addr {
		t.Errorf("addr = %q, want the default", c.Addr)
	}
}

// The guarantee inherited from the file this replaced, whose own comment
// called an unknown key "the one failure a config file invents that a
// command line never had".
func TestSuggest_NamesTheNearMiss(t *testing.T) {
	if got := Suggest("departure_grase"); got != "departure_grace" {
		t.Errorf("Suggest(\"departure_grase\") = %q, want departure_grace", got)
	}
	if got := Suggest("uploads.max_asset_sixe"); got != "uploads.max_asset_size" {
		t.Errorf("got %q, want uploads.max_asset_size", got)
	}
	// Nothing close should suggest nothing, rather than the least-bad
	// answer — "did you mean addr?" to someone who typed "hostname" is
	// worse than silence.
	if got := Suggest("completely-unrelated"); got != "" {
		t.Errorf("Suggest returned %q for a key nothing resembles", got)
	}
}

// The hub owns this number; the registry copies it rather than importing
// the hub for one constant. Two copies is fine, drift isn't: a settings
// screen would then promise a wait the server doesn't keep.
func TestDefaults_DepartureGraceMatchesTheHub(t *testing.T) {
	if got := time.Duration(Defaults().DepartureGrace); got != ws.DefaultDepartureGrace {
		t.Fatalf("registry default = %s, hub default = %s", got, ws.DefaultDepartureGrace)
	}
}
