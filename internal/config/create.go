package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// newReader is here so parse can hand the decoder an io.Reader without
// every caller thinking about it.
func newReader(data []byte) io.Reader { return bytes.NewReader(data) }

// template is the file a server writes for itself the first time it
// runs, and the first documentation of these settings a Host meets.
//
// **Written from a template rather than marshalled**, because a
// marshaller drops comments and comments are the reason this file is
// TOML at all (ADR-0006). The values come from Defaults() so the file
// can't describe behaviour the server doesn't have; the test beside this
// reads the generated file back and compares.
//
// Written once, when there is nothing there, and never rewritten. A
// Host's own comments, ordering and spacing are theirs — and a server
// that rewrote this file would take them away on the next restart. The
// cost is that a setting added later doesn't appear in an existing file,
// which is why an absent key takes its default quietly.
const template = `# Longtable server settings.
#
# Everything this server can be told is in this file, except the banner
# across the top of every page — that's set and cleared with
# "longtable set-banner" and "longtable clear-banner" instead, while the
# server keeps running, rather than edited here and restarted. See
# docs/hosting.md.
#
# Edit anything below, then restart the server for the change to take
# effect.
#
# Longtable wrote this file with its defaults because it didn't find one.
# It won't write it again. Anything you put here, including your own
# comments, stays put.

# The address to listen on. ":8080" means every address this machine
# has, which is what lets other people reach you over the network. Use
# "127.0.0.1:8080" to keep the server to this machine only.
addr = "%s"

# Where the SQLite database lives. Every room and everything in it is in
# this one file. This is the file to back up.
database = "%s"

# The directory uploaded maps and token art are stored in.
assets = "%s"

# How long someone may be disconnected before the room is told they
# left. A phone locking its screen is back well inside 30 seconds and
# nobody sees anything. On a bad network, try "2m". A real departure
# then takes that long to show.
departure_grace = "%s"
`

// create writes the default file. O_EXCL rather than a plain create: two
// servers started in the same directory at the same moment would
// otherwise race, and the loser would overwrite the winner's file — with
// identical contents today, but that is luck rather than a guarantee.
func create(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.WriteString(f, defaultFileContents()); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// defaultFileContents is the template with the defaults filled in.
func defaultFileContents() string {
	d := Defaults()
	return fmt.Sprintf(template,
		escape(d.Addr), escape(d.Database), escape(d.Assets), d.DepartureGrace)
}

// escape makes a value safe inside a TOML basic string. None of the
// defaults need it today; a Windows path in a future default would, and
// a file that won't parse is a worse first impression than any of this
// costs.
func escape(v string) string {
	return strings.NewReplacer(`\`, `\`, `"`, `\"`).Replace(v)
}
