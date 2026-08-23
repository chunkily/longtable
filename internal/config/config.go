// Package config is the Host's settings file: everything the server can
// be told, and the only place it can be told any of it.
//
// There are no setting flags and no environment variables. One file
// means a Host reads their own server's configuration by opening it,
// rather than by finding whichever shell script or service unit is
// actually starting the process — which is the thing that goes missing
// when somebody else takes over hosting, or when it's been six months.
// `-config` names a different file; that is the whole of the command
// line.
//
// The Host's banner isn't here — see cmd/longtable's `set-banner` and
// `clear-banner`. It's server *state*, changed while the process is
// running, rather than a setting the process starts with; this package
// is only ever read once, at startup, which is the wrong shape for
// something a Host wants to change without a restart.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/pelletier/go-toml/v2"
)

// DefaultPath is where the server looks when nobody says otherwise: the
// working directory, beside the database and asset directory it also
// defaults to, so everything one server owns sits in one folder the Host
// chose.
const DefaultPath = "longtable.toml"

// Duration is a time.Duration written the way a person writes one —
// `"30s"`, `"2m"`. TOML has no duration type, and a bare number would
// have to mean nanoseconds or seconds by convention, which is exactly
// the sort of thing a Host would have to leave the file to look up.
type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		// time's own message quotes the input and names the units it
		// knows, which is more use to a Host than anything shorter.
		return err
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) String() string { return time.Duration(d).String() }

// ByteSize is a count of bytes written the way a person writes one —
// `"25 MiB"`, `"5 GB"` — for the same reason Duration exists: TOML has
// no size type, and a bare number would leave a Host doing the
// arithmetic by hand to know what they'd typed.
type ByteSize int64

func (b *ByteSize) UnmarshalText(text []byte) error {
	parsed, err := humanize.ParseBytes(string(text))
	if err != nil {
		return err
	}
	*b = ByteSize(parsed)
	return nil
}

// String renders in IEC units (MiB, GiB) rather than SI (MB, GB) to
// match how the doc comments and this file's own defaults talk about
// sizes — and because "MB" is ambiguous between the two in a way nobody
// editing a config file benefits from.
func (b ByteSize) String() string { return humanize.IBytes(uint64(b)) }

// Config is the whole of a Longtable server's settings. The banner
// isn't one of them — see the package doc for why.
//
// Flat keys, plus the one `[section]`: upload limits turned out to be
// the settings that didn't belong beside these, so they nest under
// `Uploads` instead of flattening into `max_asset_size` and
// `max_total_storage` at the top level. TOML puts every top-level key
// *above* the first table header, so the generated file keeps `Uploads`
// last.
type Config struct {
	Addr           string       `toml:"addr"`
	Database       string       `toml:"database"`
	Assets         string       `toml:"assets"`
	DepartureGrace Duration     `toml:"departure_grace"`
	Uploads        UploadLimits `toml:"uploads"`
}

// UploadLimits bounds what an upload can cost a Host's disk. Both guard
// different things: MaxAssetSize stops any one upload being unreasonably
// large, MaxTotalStorage stops the sum of everything ever uploaded from
// growing without bound. Enforced in internal/api's upload handler.
type UploadLimits struct {
	MaxAssetSize    ByteSize `toml:"max_asset_size"`
	MaxTotalStorage ByteSize `toml:"max_total_storage"`
}

// Defaults is what a server runs on when its file says nothing, and what
// the generated file is written from — one source for both, so a fresh
// file can't drift from the behaviour it describes.
//
// DepartureGrace mirrors ws.DefaultDepartureGrace rather than importing
// it: this package is a leaf, and pulling the hub in for one constant
// would drag the store and the socket library into anything that reads a
// setting. The test beside this file fails if the two ever disagree.
func Defaults() Config {
	return Config{
		Addr:           ":8080",
		Database:       "longtable.db",
		Assets:         "longtable-assets",
		DepartureGrace: Duration(30 * time.Second),
		Uploads: UploadLimits{
			// Matches the ceiling this server enforced before it was
			// configurable — generous for map/token art.
			MaxAssetSize: ByteSize(25 * humanize.MiByte),
			// Enough for a well-stocked room or two before a Host has to
			// think about it, without leaving a fresh install able to
			// swallow a disk on its own.
			MaxTotalStorage: ByteSize(10 * humanize.GiByte),
		},
	}
}

// Load reads the config at path, which has to be there.
//
// For a path a Host named themselves. Somebody who types `-config
// server.toml` has a file in mind, and creating a fresh default one at a
// mistyped path would look exactly like their settings being ignored:
// the server starts, and everything they configured is quietly gone.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("no config file at %s", path)
		}
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}
	return parse(path, data)
}

// LoadOrCreate reads the config at path, writing one full of defaults
// first if there is nothing there.
//
// For the default location only, where a missing file means a server
// that has never been configured rather than a Host pointing at the
// wrong thing. The file it writes is the documentation a Host sees
// first, so it carries a comment per setting.
func LoadOrCreate(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := create(path); err != nil {
			return Config{}, err
		}
		return Defaults(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}
	return parse(path, data)
}

// parse turns a file's bytes into settings, refusing anything it can't
// account for.
//
// **An unknown key is an error, not something to ignore.** A Host who
// mistypes `departure_grase` otherwise edits the file, restarts, and
// watches the setting do nothing, with the server logging its usual
// cheerful startup line — the one failure a config file introduces that
// a command line never had. Startup is the moment they are watching, so
// it is the moment to say so.
//
// A key that is simply *absent* takes its default and says nothing.
// That is what lets a version add a setting without breaking every file
// already out there, and it is why the generated file is written once
// and never rewritten.
func parse(path string, data []byte) (Config, error) {
	cfg := Defaults()

	dec := toml.NewDecoder(newReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		var unknown *toml.StrictMissingError
		if errors.As(err, &unknown) {
			// String() draws the offending line with the key underlined,
			// which is the difference between a Host finding their typo and
			// hunting for it.
			return Config{}, fmt.Errorf("%s: unknown setting\n%s", path, unknown.String())
		}
		var decErr *toml.DecodeError
		if errors.As(err, &decErr) {
			// String() carries the summary under the line it points at, so
			// wrapping the error as well would say it twice.
			return Config{}, fmt.Errorf("%s:\n%s", path, decErr.String())
		}
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}

	return cfg, cfg.validate(path)
}

// validate catches the values that parse cleanly and can't be run with.
// Each one would otherwise fail much later and somewhere less obvious —
// an empty database path as a confusing SQLite error, a zero grace
// period as everybody's badge flickering on the first phone to sleep.
func (c Config) validate(path string) error {
	// Plain sentences, no em dashes: a Host reads these, and the register
	// for that is docs/, not the comments around them. See the
	// longtable-copy skill.
	switch {
	case c.Addr == "":
		return fmt.Errorf("%s: addr can't be empty. It's the address to listen on, like \":8080\"", path)
	case c.Database == "":
		return fmt.Errorf("%s: database can't be empty. It's the path to the SQLite file", path)
	case c.Assets == "":
		return fmt.Errorf("%s: assets can't be empty. It's the directory uploaded images go in", path)
	case time.Duration(c.DepartureGrace) <= 0:
		return fmt.Errorf("%s: departure_grace has to be longer than zero, like \"30s\"", path)
	case c.Uploads.MaxAssetSize <= 0:
		return fmt.Errorf("%s: uploads.max_asset_size has to be more than zero, like \"25 MiB\"", path)
	case c.Uploads.MaxTotalStorage <= 0:
		return fmt.Errorf("%s: uploads.max_total_storage has to be more than zero, like \"10 GiB\"", path)
	case c.Uploads.MaxTotalStorage < c.Uploads.MaxAssetSize:
		return fmt.Errorf("%s: uploads.max_total_storage can't be smaller than uploads.max_asset_size — nothing could ever be uploaded", path)
	}
	return nil
}
