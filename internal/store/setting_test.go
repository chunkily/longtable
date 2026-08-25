package store

import "testing"

func TestSettings_RoundTrip(t *testing.T) {
	s := newTestStore(t)

	values, err := s.Settings()
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	// A server nobody has configured stores nothing at all — every value
	// it runs on comes from the registry's defaults.
	if len(values) != 0 {
		t.Fatalf("a fresh database already holds %d settings", len(values))
	}

	if err := s.SetSetting("departure_grace", "2m0s"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	values, err = s.Settings()
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if values["departure_grace"] != "2m0s" {
		t.Fatalf("departure_grace = %q, want 2m0s", values["departure_grace"])
	}
}

// "Unset, so it's on the default" and "set to exactly this" are
// different answers, and a CLI that collapsed them would leave a Host
// unable to tell whether they had changed anything.
func TestGetSetting_SaysWhetherItWasSet(t *testing.T) {
	s := newTestStore(t)

	if _, ok, err := s.GetSetting("addr"); err != nil || ok {
		t.Fatalf("GetSetting on an unset key: ok = %v, err = %v", ok, err)
	}

	if err := s.SetSetting("addr", ":9000"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	value, ok, err := s.GetSetting("addr")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if !ok || value != ":9000" {
		t.Fatalf("GetSetting = (%q, %v), want (\":9000\", true)", value, ok)
	}
}

// Writing the same key twice replaces it rather than failing, because
// the server and a `longtable config set` can hold this file open at the
// same moment — the same race FindOrCreateAsset is written around.
func TestSetSetting_Replaces(t *testing.T) {
	s := newTestStore(t)

	for _, value := range []string{":9000", ":9001", ":9002"} {
		if err := s.SetSetting("addr", value); err != nil {
			t.Fatalf("SetSetting(%q): %v", value, err)
		}
	}

	value, _, err := s.GetSetting("addr")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if value != ":9002" {
		t.Fatalf("addr = %q, want the last one written", value)
	}
}

func TestUnsetSetting_PutsItBackToAbsent(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetSetting("addr", ":9000"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if err := s.UnsetSetting("addr"); err != nil {
		t.Fatalf("UnsetSetting: %v", err)
	}
	if _, ok, _ := s.GetSetting("addr"); ok {
		t.Fatal("addr is still set")
	}

	// Unsetting what was never set is the state the caller asked for, not
	// an error to report.
	if err := s.UnsetSetting("addr"); err != nil {
		t.Fatalf("unsetting an absent key: %v", err)
	}
}
