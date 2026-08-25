// Package config is the registry of everything a Host can configure:
// every setting there is, what it defaults to, what it means, and the
// rules a value has to pass before it is stored.
//
// It holds no values of its own and reads nothing. Settings live in the
// database ([ADR-0009]), and this package is the leaf that says what a
// setting *is* — so `longtable config`, the admin UI's fields and help
// text, the comments in an exported file, and the validation on the way
// in all read one table. That is what keeps the CLI and the UI from
// drifting into two different ideas of what can be configured.
//
// Two things are deliberately not here. The database's own location
// cannot be a setting, because nothing could read it — it is the `-db`
// flag. And the Host's banner is server state rather than configuration:
// see cmd/longtable's `set-banner`, and the note in settings-in-database
// about whether it should move.
//
// [ADR-0009]: ../../planning/decisions/0009-admin-ui-and-database-config.md
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// Duration is a time.Duration written the way a person writes one —
// `"30s"`, `"2m"`. A bare number would have to mean nanoseconds or
// seconds by convention, which is exactly the sort of thing a Host would
// have to go and look up.
type Duration time.Duration

func (d Duration) String() string { return time.Duration(d).String() }

// ByteSize is a count of bytes written the way a person writes one —
// `"25 MiB"`, `"5 GB"` — for the same reason Duration exists: nobody
// should be counting zeroes to say how big an upload may be.
type ByteSize int64

// String renders in IEC units (MiB, GiB) rather than SI, because "MB" is
// ambiguous between the two in a way nobody choosing a limit benefits
// from.
func (b ByteSize) String() string { return humanize.IBytes(uint64(b)) }

// Config is a server's settings once they have been read and resolved —
// what the rest of the program is handed. Anything missing from the
// database takes the registry's default, which is what lets a version
// add a setting without every existing database having to know about it.
type Config struct {
	Addr           string
	Assets         string
	DepartureGrace Duration
	Uploads        UploadLimits
}

// UploadLimits bounds what an upload can cost a Host's disk.
// MaxAssetSize stops any one upload being unreasonably large;
// MaxTotalStorage stops the sum of everything ever uploaded from growing
// without bound.
type UploadLimits struct {
	MaxAssetSize    ByteSize
	MaxTotalStorage ByteSize
}

// Setting is one thing a Host can change, and the only place its
// default, its meaning and its rules live.
type Setting struct {
	// Key is what a Host types and what the database stores. Dotted
	// rather than nested, because that is how the value was already
	// spelled when written inline in the file this replaced.
	Key string

	// Default is the canonical rendering of the value used when the
	// database says nothing.
	Default string

	// Summary is the one line that used to be a comment above the setting
	// in longtable.toml. Deleting that file made this the only place a
	// Host is told what a setting does, so it is not decoration — see
	// ADR-0006, which chose a commentable format for exactly this and is
	// superseded by the decision that created this package.
	Summary string

	// Live settings take effect when they are written. The rest need a
	// restart, and both the CLI and the UI have to say so rather than
	// letting a save imply a change that hasn't happened.
	//
	// Everything is false today. Live is per-setting plumbing — the
	// banner manages it by re-reading on every request — and claiming it
	// before the plumbing exists would be the one lie a settings screen
	// must not tell.
	Live bool

	// Parse validates a raw value and returns the form to store, so what
	// `set` accepts is what `get` prints back: "26214400" comes out
	// "25 MiB".
	Parse func(raw string) (string, error)

	// apply puts an already-canonical value into a Config. Unexported
	// because nothing outside resolves a single setting on its own.
	apply func(c *Config, canonical string) error
}

// Registry is every setting there is, in the order a Host should meet
// them. Adding a row is the whole of adding a setting: the CLI, an
// export and the admin UI all read this.
var Registry = []Setting{
	stringSetting(
		"addr", ":8080",
		`The address to listen on. ":8080" means every address this machine has, which is what lets other people reach you over the network. Use "127.0.0.1:8080" to keep the server to this machine only.`,
		func(c *Config, v string) { c.Addr = v },
	),
	stringSetting(
		"assets", "longtable-assets",
		"The directory uploaded maps and token art go in. This is the second half of a backup — the database does not contain it.",
		func(c *Config, v string) { c.Assets = v },
	),
	durationSetting(
		"departure_grace", 30*time.Second,
		`How long someone may be disconnected before the room is told they left. A phone locking its screen is back well inside 30 seconds and nobody sees anything. On a bad network, try "2m".`,
		func(c *Config, v Duration) { c.DepartureGrace = v },
	),
	byteSizeSetting(
		"uploads.max_asset_size", 25*humanize.MiByte,
		"The largest a single map or token upload may be.",
		func(c *Config, v ByteSize) { c.Uploads.MaxAssetSize = v },
	),
	byteSizeSetting(
		"uploads.max_total_storage", 10*humanize.GiByte,
		"The most this server will ever store across every room's uploads combined. Art shared by several rooms only counts once.",
		func(c *Config, v ByteSize) { c.Uploads.MaxTotalStorage = v },
	),
}

// Lookup finds a setting by key.
func Lookup(key string) (Setting, bool) {
	for _, s := range Registry {
		if s.Key == key {
			return s, true
		}
	}
	return Setting{}, false
}

// Keys is every setting's key, in registry order.
func Keys() []string {
	keys := make([]string, 0, len(Registry))
	for _, s := range Registry {
		keys = append(keys, s.Key)
	}
	return keys
}

// Resolve turns stored values into a Config, filling anything absent
// from the registry's defaults.
//
// An unknown key in the map is ignored rather than refused. Refusal is
// the job of the write path, where a person is present to be told; a key
// left behind by an older version must not stop a server starting.
func Resolve(values map[string]string) (Config, error) {
	var c Config
	for _, s := range Registry {
		raw, ok := values[s.Key]
		if !ok || raw == "" {
			raw = s.Default
		}
		if err := s.apply(&c, raw); err != nil {
			// Only reachable if something wrote past Parse — a hand-edited
			// database, most likely. Name the key, since the value alone
			// gives a Host nothing to go on.
			return Config{}, fmt.Errorf("%s: %w", s.Key, err)
		}
	}
	return c, nil
}

// Defaults is what a server runs on when the database says nothing.
//
// Derived from the registry rather than written out a second time, so a
// default cannot disagree with the description printed beside it.
func Defaults() Config {
	c, err := Resolve(nil)
	if err != nil {
		// A registry whose own defaults don't parse is a programming
		// error, not a Host's. TestRegistry_DefaultsAreValid says so
		// before this can reach anyone.
		panic("config: registry defaults do not parse: " + err.Error())
	}
	return c
}

// Validate catches what no single setting can judge on its own.
//
// Run on the way in as well as at startup, so a value that cannot work
// is refused while the person who typed it is still looking at it —
// which is the main thing the database bought over a file, where the
// same mistake surfaced as a server that wouldn't boot.
func (c Config) Validate() error {
	// Plain sentences: a Host reads these. See the longtable-copy skill.
	if c.Uploads.MaxTotalStorage < c.Uploads.MaxAssetSize {
		return fmt.Errorf(
			"uploads.max_total_storage can't be smaller than uploads.max_asset_size (%s). Nothing could ever be uploaded",
			c.Uploads.MaxAssetSize)
	}
	return nil
}

// Suggest names the setting a mistyped key was probably meant to be, or
// "" when nothing is close.
//
// The file this replaced refused an unknown key and drew the line it was
// on, and its own comment called that "the one failure a config file
// invents that a command line never had". Deleting the file must not
// delete the guarantee.
func Suggest(key string) string {
	best, bestDistance := "", 0
	for _, candidate := range Keys() {
		d := distance(key, candidate)
		// Three edits is about where a suggestion stops being helpful and
		// starts being a guess.
		if d > 3 {
			continue
		}
		if best == "" || d < bestDistance {
			best, bestDistance = candidate, d
		}
	}
	return best
}

// distance is Levenshtein, one row at a time.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, min(curr[j-1]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// The three constructors below exist so a registry entry says what a
// setting is and nothing about how its type is handled. Each builds both
// Parse and apply from one parser, which is what stops the two
// disagreeing about what a valid value is.

func stringSetting(key, def, summary string, assign func(*Config, string)) Setting {
	return Setting{
		Key: key, Default: def, Summary: summary,
		Parse: func(raw string) (string, error) {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return "", fmt.Errorf("%s can't be empty", key)
			}
			return raw, nil
		},
		apply: func(c *Config, canonical string) error {
			if strings.TrimSpace(canonical) == "" {
				return fmt.Errorf("can't be empty")
			}
			assign(c, canonical)
			return nil
		},
	}
}

func durationSetting(key string, def time.Duration, summary string, assign func(*Config, Duration)) Setting {
	parse := func(raw string) (time.Duration, error) {
		d, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			// time's own message names the units it knows, which is more
			// use than anything shorter would be.
			return 0, fmt.Errorf("%q isn't a length of time. Try %q", raw, "30s")
		}
		if d <= 0 {
			return 0, fmt.Errorf("has to be longer than zero, like %q", "30s")
		}
		return d, nil
	}
	return Setting{
		Key: key, Default: Duration(def).String(), Summary: summary,
		Parse: func(raw string) (string, error) {
			d, err := parse(raw)
			if err != nil {
				return "", err
			}
			return Duration(d).String(), nil
		},
		apply: func(c *Config, canonical string) error {
			d, err := parse(canonical)
			if err != nil {
				return err
			}
			assign(c, Duration(d))
			return nil
		},
	}
}

func byteSizeSetting(key string, def uint64, summary string, assign func(*Config, ByteSize)) Setting {
	parse := func(raw string) (ByteSize, error) {
		n, err := humanize.ParseBytes(strings.TrimSpace(raw))
		if err != nil {
			return 0, fmt.Errorf("%q isn't a size. Try %q or %q", raw, "25 MiB", "2 GB")
		}
		if n == 0 {
			return 0, fmt.Errorf("has to be more than zero, like %q", "25 MiB")
		}
		return ByteSize(n), nil
	}
	return Setting{
		Key: key, Default: ByteSize(def).String(), Summary: summary,
		Parse: func(raw string) (string, error) {
			n, err := parse(raw)
			if err != nil {
				return "", err
			}
			return n.String(), nil
		},
		apply: func(c *Config, canonical string) error {
			n, err := parse(canonical)
			if err != nil {
				return err
			}
			assign(c, n)
			return nil
		},
	}
}
