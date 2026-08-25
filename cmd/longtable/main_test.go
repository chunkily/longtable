package main

import (
	"strings"
	"testing"
)

// The help is the only place a Host is told these commands exist —
// nothing else enumerates them, and `longtable room list` in particular
// is the sole way to see the rooms on a server. A command that stops
// being listed is a command nobody finds.
func TestUsage_NamesEveryCommand(t *testing.T) {
	for _, command := range []string{
		"serve",
		"config list",
		"config set",
		"room list",
		"room reset-password",
		"set-banner",
		"clear-banner",
	} {
		if !strings.Contains(usage, command) {
			t.Errorf("help doesn't mention %q", command)
		}
	}
}

// The gap this whole thing exists to close: the flag package answers -h
// with the flags of one FlagSet, so `-db` alone was the entire published
// surface of a CLI with five commands.
func TestUsage_SaysMoreThanTheConfigFlag(t *testing.T) {
	if !strings.Contains(usage, "-db") {
		t.Error("help doesn't mention the flag that says which database")
	}
	if strings.Count(usage, "\n") < 10 {
		t.Errorf("help is too short to be listing commands:\n%s", usage)
	}
}

// A command that needs an argument has to show it. This is the half the
// flag package cannot do: it prints -db and stops, which reads as
// "takes no arguments" for exactly the two commands that do.
func TestUsage_ShowsTheArgumentsCommandsRequire(t *testing.T) {
	for _, placeholder := range []string{"<room-code>", "<message>"} {
		if !strings.Contains(usage, placeholder) {
			t.Errorf("help doesn't show the %s argument", placeholder)
		}
	}
}

// Running the bare binary has to keep starting a server: the hosting
// guide tells a Host to double-click it, so anything that turned a
// no-argument run into a help screen would break the first thing they
// are told to do.
func TestIsHelpArg(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		if !isHelpArg(arg) {
			t.Errorf("%q should ask for help", arg)
		}
	}
	// "serve" is the one that matters here: it is what a bare run means,
	// and treating it as help would stop the server starting.
	for _, arg := range []string{"serve", "room", "config", "set-banner", "-db", "", "helper"} {
		if isHelpArg(arg) {
			t.Errorf("%q should not be read as a request for help", arg)
		}
	}
}

// The binary doesn't send anyone to a file it can't promise is there. It
// is downloaded on its own as often as it arrives beside the repo, and
// naming a path that isn't there is worse than naming nothing.
func TestUsage_SendsNobodyToAFileItCannotPromise(t *testing.T) {
	for _, reference := range []string{"docs/", ".md", "README"} {
		if strings.Contains(usage, reference) {
			t.Errorf("help points at %q, which the binary can't guarantee exists", reference)
		}
	}
}

// The rule worth testing here is the one that is easy to get backwards.
// A bare run and a bare flag both mean "serve"; a bare *word* that isn't
// a command means someone typed something wrong, and serving anyway
// hides that completely.
func TestRouteFor(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want route
	}{
		// The double-click case, and the reason routeFor can't simply
		// reject anything it doesn't recognise.
		{"no arguments at all", nil, routeServe},
		{"an explicit serve", []string{"serve"}, routeServe},
		{"serve with its flag", []string{"serve", "-db", "other.db"}, routeServe},
		// A flag with no command in front of it has always been serve's.
		{"a bare flag", []string{"-db", "other.db"}, routeServe},
		{"a bare flag, long form", []string{"--db", "other.db"}, routeServe},

		{"room", []string{"room", "list"}, routeRoom},
		{"set-banner", []string{"set-banner", "back at 9"}, routeSetBanner},
		{"clear-banner", []string{"clear-banner"}, routeClearBanner},
		{"config", []string{"config", "list"}, routeConfig},

		{"-h", []string{"-h"}, routeHelp},
		{"--help", []string{"--help"}, routeHelp},
		{"help", []string{"help"}, routeHelp},

		// The whole point: these used to start a server.
		{"a misspelled command", []string{"serv"}, routeUnknown},
		{"a command that doesn't exist", []string{"rooms"}, routeUnknown},
		{"the wrong case", []string{"Room"}, routeUnknown},
		{"a stray word before a flag", []string{"srv", "-db", "other.db"}, routeUnknown},
	} {
		if got := routeFor(tc.args); got != tc.want {
			t.Errorf("%s: routeFor(%q) = %q, want %q", tc.name, tc.args, got, tc.want)
		}
	}
}

// The top-level help names the two settings commands anyone runs daily
// and points at "longtable config -h" for the rest, so that is where the
// other four have to actually be.
func TestConfigUsage_NamesEverySettingsCommand(t *testing.T) {
	for _, command := range []string{"list", "get", "set", "unset", "export", "import"} {
		if !strings.Contains(configUsage, "config "+command) {
			t.Errorf("config help doesn't mention %q", command)
		}
	}
}

// Both halves of `room`, since a Host who runs `room` alone has just
// demonstrated they don't know which subcommand they wanted.
func TestRoomUsage_NamesBothSubcommands(t *testing.T) {
	for _, subcommand := range []string{"list", "reset-password", "<room-code>"} {
		if !strings.Contains(roomUsage, subcommand) {
			t.Errorf("room help doesn't mention %q", subcommand)
		}
	}
}
