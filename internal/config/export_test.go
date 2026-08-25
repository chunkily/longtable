package config

import (
	"strings"
	"testing"
)

// The point of an export: a Host gets one artefact back that means what
// their server means, and importing it somewhere else reproduces it.
func TestExportImport_RoundTrips(t *testing.T) {
	stored := map[string]string{
		"addr":                   ":9000",
		"departure_grace":        "2m0s",
		"uploads.max_asset_size": "50 MiB",
	}

	values, err := Import([]byte(Export(stored)))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	for key, want := range stored {
		if values[key] != want {
			t.Errorf("%s came back %q, want %q", key, values[key], want)
		}
	}

	// An export writes every setting, not only the changed ones, so a
	// Host reading the file sees the whole picture and an import doesn't
	// silently leave a value behind.
	for _, key := range Keys() {
		if _, ok := values[key]; !ok {
			t.Errorf("%s is missing from the export", key)
		}
	}
}

// Anything not stored exports as its default, so the file describes the
// server rather than only the parts of it somebody has touched.
func TestExport_WritesDefaultsForWhatIsUnset(t *testing.T) {
	values, err := Import([]byte(Export(nil)))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	for _, s := range Registry {
		if values[s.Key] != s.Default {
			t.Errorf("%s exported as %q, want its default %q", s.Key, values[s.Key], s.Default)
		}
	}
}

// The trap this file exists to not fall into. TOML puts every bare key
// above the first [section] header, so a top-level setting written after
// one silently becomes part of it — `addr` would import back as
// `uploads.addr` and the real `addr` would take its default with nothing
// to say it had.
func TestExport_PutsBareKeysAboveTheFirstSection(t *testing.T) {
	out := Export(nil)
	firstSection := strings.Index(out, "\n[")
	if firstSection == -1 {
		t.Skip("no sections in the registry yet")
	}
	for _, s := range Registry {
		if strings.Contains(s.Key, ".") {
			continue
		}
		at := strings.Index(out, "\n"+s.Key+" =")
		if at == -1 {
			t.Errorf("%s isn't in the export", s.Key)
			continue
		}
		if at > firstSection {
			t.Errorf("%s is written after a [section] header, so it will import as part of it", s.Key)
		}
	}
}

// ADR-0006 chose a commentable format so a Host could see what a setting
// does without leaving the file. The file it chose that for is gone, and
// this is where that debt is paid.
func TestExport_ExplainsEverySetting(t *testing.T) {
	out := Export(nil)
	for _, s := range Registry {
		// The first few words are enough: the summary is wrapped across
		// comment lines, so the whole string won't appear verbatim.
		opening := strings.Join(strings.Fields(s.Summary)[:3], " ")
		if !strings.Contains(out, opening) {
			t.Errorf("%s is exported with no explanation", s.Key)
		}
	}
	// A Host reading the file has to know which changes are waiting on a
	// restart, since none of them are live yet.
	if !strings.Contains(out, "needs a restart") {
		t.Error("nothing in the export says a change needs a restart")
	}
}

// Values arrive as text and are judged by the registry, so an import
// cannot put something into the database that `config set` would have
// refused. Unquoted numbers are the likely case: a Host writing
// `departure_grace = 30` meant something by it.
func TestImport_ReadsUnquotedValuesAsText(t *testing.T) {
	values, err := Import([]byte("departure_grace = 30\n"))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if values["departure_grace"] != "30" {
		t.Fatalf("got %q, want \"30\"", values["departure_grace"])
	}
	// And it is refused at the point it would be stored, because "30" is
	// not a length of time.
	s, _ := Lookup("departure_grace")
	if _, err := s.Parse(values["departure_grace"]); err == nil {
		t.Error("a bare number was accepted as a duration")
	}
}

func TestImport_RefusesBrokenTOML(t *testing.T) {
	if _, err := Import([]byte("addr = \n")); err == nil {
		t.Fatal("broken TOML was accepted")
	}
}
