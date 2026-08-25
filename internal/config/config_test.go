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
