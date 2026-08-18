package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"longtable/internal/ws"
)

func write(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "longtable.toml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// The generated file is the only documentation of these settings a Host
// is guaranteed to see, so it has to describe the server they are
// actually running. Reading it back is the check that it does.
func TestLoadOrCreate_WritesAFileThatMeansTheDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "longtable.toml")

	created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if created != Defaults() {
		t.Fatalf("returned %+v, want the defaults %+v", created, Defaults())
	}

	reread, err := Load(path)
	if err != nil {
		t.Fatalf("reading back the generated file: %v", err)
	}
	if reread != Defaults() {
		t.Fatalf("the generated file means %+v, want the defaults %+v", reread, Defaults())
	}
}

// ADR-0006 chose TOML over JSON for exactly one reason.
func TestLoadOrCreate_TheGeneratedFileExplainsEverySetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "longtable.toml")
	if _, err := LoadOrCreate(path); err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	contents := string(data)
	for _, key := range []string{"addr", "database", "assets", "banner", "departure_grace"} {
		if !strings.Contains(contents, key+" = ") {
			t.Errorf("generated file has no %q setting", key)
		}
	}
	if strings.Count(contents, "#") < 5 {
		t.Errorf("generated file has fewer comments than settings:\n%s", contents)
	}
}

// A second run reads the Host's file. It must not be rewritten — their
// comments and their ordering are theirs.
func TestLoadOrCreate_LeavesAnExistingFileAlone(t *testing.T) {
	path := write(t, "# my own note\naddr = \":9000\"\n")

	cfg, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}
	if cfg.Addr != ":9000" {
		t.Errorf("addr = %q, want the file's :9000", cfg.Addr)
	}
	// Everything it didn't mention still works.
	if cfg.Database != Defaults().Database {
		t.Errorf("database = %q, want the default %q", cfg.Database, Defaults().Database)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "# my own note") {
		t.Errorf("the Host's own comment was rewritten away:\n%s", data)
	}
}

// A path a Host typed themselves is a file they believe in. Creating a
// fresh one there looks identical to their settings being ignored.
func TestLoad_RefusesAPathThatIsNotThere(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.toml")

	if _, err := Load(path); err == nil {
		t.Fatal("Load succeeded on a missing file, want an error")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("Load created the file it was asked to read")
	}
}

// The failure a config file introduces that a command line never had:
// an edit that silently does nothing.
func TestLoad_RefusesAnUnknownSetting(t *testing.T) {
	path := write(t, "addr = \":8080\"\ndeparture_grase = \"2s\"\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("a mistyped setting was accepted")
	}
	if !strings.Contains(err.Error(), "departure_grase") {
		t.Errorf("the error doesn't name the offending key: %v", err)
	}
}

func TestLoad_RefusesWhatItCannotRun(t *testing.T) {
	for name, contents := range map[string]string{
		"broken TOML":                "addr = \n",
		"a duration that is not one": "departure_grace = \"soon\"\n",
		"a negative duration":        "departure_grace = \"-5s\"\n",
		"an empty database path":     "database = \"\"\n",
		"an empty address":           "addr = \"\"\n",
		"an empty assets directory":  "assets = \"\"\n",
	} {
		if _, err := Load(write(t, contents)); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

// Settings a file doesn't mention take their defaults rather than their
// zero values — which is what lets a later version add one without
// breaking every file already out there.
func TestLoad_ReadsWhatIsThereAndDefaultsTheRest(t *testing.T) {
	path := write(t, "banner = \"back at 9\"\ndeparture_grace = \"2m\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Banner != "back at 9" {
		t.Errorf("banner = %q", cfg.Banner)
	}
	if time.Duration(cfg.DepartureGrace) != 2*time.Minute {
		t.Errorf("departure_grace = %s, want 2m", cfg.DepartureGrace)
	}
	if cfg.Addr != Defaults().Addr || cfg.Assets != Defaults().Assets {
		t.Errorf("an unmentioned setting lost its default: %+v", cfg)
	}
}

// The hub owns this number; this package copies it rather than importing
// the hub for one constant. Two copies is fine, drift isn't: the
// generated file would then promise a wait the server doesn't keep.
func TestDefaults_DepartureGraceMatchesTheHub(t *testing.T) {
	if got := time.Duration(Defaults().DepartureGrace); got != ws.DefaultDepartureGrace {
		t.Fatalf("config default = %s, hub default = %s", got, ws.DefaultDepartureGrace)
	}
}
