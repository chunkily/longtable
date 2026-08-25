// Command longtable is the self-hosted Longtable server: a single binary
// that serves the frontend, the API, and the real-time sync socket, and
// persists to an embedded SQLite database.
//
// Usage:
//
//	longtable [serve] [-config longtable.toml]
//	longtable room list [-config longtable.toml]
//	longtable room reset-password [-config longtable.toml] <room-code>
//	longtable set-banner [-config longtable.toml] <message>
//	longtable clear-banner [-config longtable.toml]
//
// `longtable -h` prints the same list.
//
// Every setting lives in the config file, which the server writes for
// itself the first time it runs. `-config` is the only flag any
// subcommand takes: see internal/config for why.
//
// set-banner and clear-banner are the exception to "settings live in
// the file" — they change a message across the top of every page while
// the server keeps running, by writing straight to its database the way
// room reset-password already does. See docs/hosting.md.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	assets "longtable"
	"longtable/internal/api"
	"longtable/internal/blobstore"
	"longtable/internal/config"
	"longtable/internal/db"
	"longtable/internal/lanurl"
	"longtable/internal/store"
	"longtable/internal/ws"
)

// usage is what `longtable -h` prints.
//
// Written out here rather than assembled from the flag package's own
// output, which only ever knows about one subcommand's flags: asked
// "what can this do?", it answers with a single -config line and no
// mention that `room` or `set-banner` exist at all.
//
// Each command's arguments are shown next to it, because the flag
// package can't: `longtable set-banner -h` lists -config and stops,
// which reads as "takes no arguments" for the two commands that need
// one.
const usage = `Longtable is a self-hosted virtual tabletop for D&D.

Usage:
  longtable [command] [-config longtable.toml]

With no command, Longtable starts the server.

Commands:
  serve                            Start the server.
  room list                        List every room on this server.
  room reset-password <room-code>  Set a new GM password for a room.
  set-banner <message>             Show a message across the top of every page.
  clear-banner                     Take that message down.

Flags:
  -config <path>                   The settings file to read. Default: longtable.toml
`

// isHelpArg covers the three spellings people actually type. `help` is
// in there because it is a command in most CLIs, and it would otherwise
// be ignored as a stray argument and start a server.
func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

// route is what a set of arguments asks for.
//
// Split out of main and given no side effects so the one rule here that
// is easy to get wrong can be tested without starting a server: which
// arguments mean "serve" and which mean "you typed something wrong".
// Those two sit closer together than they look — a bare `longtable`
// serves, and so does `longtable -config other.toml`, but `longtable
// serv` must not.
type route string

const (
	routeHelp        route = "help"
	routeServe       route = "serve"
	routeRoom        route = "room"
	routeSetBanner   route = "set-banner"
	routeClearBanner route = "clear-banner"
	routeUnknown     route = "unknown"
)

func routeFor(args []string) route {
	if len(args) == 0 {
		// The double-click case. docs/hosting.md tells a Host to run the
		// binary and nothing else, so this has to stay the default.
		return routeServe
	}

	switch {
	case isHelpArg(args[0]):
		return routeHelp
	case args[0] == "serve":
		return routeServe
	case args[0] == "room":
		return routeRoom
	case args[0] == "set-banner":
		return routeSetBanner
	case args[0] == "clear-banner":
		return routeClearBanner
	case strings.HasPrefix(args[0], "-"):
		// A flag with no command in front of it belongs to serve, which is
		// what `longtable -config other.toml` has always meant.
		return routeServe
	default:
		return routeUnknown
	}
}

func main() {
	args := os.Args[1:]

	var err error
	switch routeFor(args) {
	case routeHelp:
		// Never an error: someone who typed --help asked a question and
		// got an answer, so it goes to stdout and exits 0.
		fmt.Print(usage)
	case routeRoom:
		err = runRoomCommand(args[1:])
	case routeSetBanner:
		err = runSetBanner(args[1:])
	case routeClearBanner:
		err = runClearBanner(args[1:])
	case routeUnknown:
		// Refused rather than served. A mistyped command used to fall
		// through to `serve` and come up looking like a normal start,
		// which is the same failure the config parser refuses to make:
		// the Host asked for something, got a running server, and nothing
		// anywhere said the thing they typed doesn't exist.
		err = fmt.Errorf("unknown command %q. Run longtable -h to see the commands", args[0])
	default:
		if len(args) > 0 && args[0] == "serve" {
			args = args[1:]
		}
		err = runServe(args)
	}

	if err != nil {
		// Plain text on stderr rather than slog, which quotes a message
		// onto one line: the config parser answers a typo by drawing the
		// offending line with the key underlined, and slog turns that into
		// a string full of escaped newlines. This is the last thing a Host
		// sees before the process goes, so it is the one place worth
		// printing rather than logging.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServe(args []string) error {
	fset := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := addConfigFlag(fset, "longtable [serve] [-config path]")
	fset.Parse(args)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	return serve(cfg)
}

func serve(cfg config.Config) error {
	s, closeDB, err := openStore(cfg.Database)
	if err != nil {
		return err
	}
	defer closeDB()

	blobs, err := blobstore.New(cfg.Assets)
	if err != nil {
		return fmt.Errorf("open asset storage: %w", err)
	}

	frontend, err := fs.Sub(assets.Dist, assets.DistDir)
	if err != nil {
		return fmt.Errorf("load embedded frontend: %w", err)
	}

	hub := ws.NewHub(s, time.Duration(cfg.DepartureGrace))
	limits := api.UploadLimits{
		MaxAssetBytes:   int64(cfg.Uploads.MaxAssetSize),
		MaxTotalStorage: int64(cfg.Uploads.MaxTotalStorage),
	}
	router := api.NewRouter(s, hub, blobs, frontend, limits)

	slog.Info("longtable: listening", "addr", cfg.Addr, "db", cfg.Database, "assets", cfg.Assets)
	logReachableURLs(cfg.Addr)
	return http.ListenAndServe(cfg.Addr, router)
}

// addConfigFlag registers the one flag every subcommand has: settings
// live in the file, and this says which file.
//
// synopsis is the command's own usage line, and it is here rather than
// left to the flag package because that prints the flags and nothing
// else — a command with a required argument comes out looking like a
// command with none. Passing it in also means the same string can be
// the error text when the argument is missing, so the two can't
// disagree about what the command takes.
func addConfigFlag(fset *flag.FlagSet, synopsis string) *string {
	fset.Usage = func() {
		fmt.Fprintf(fset.Output(), "Usage: %s\n\nFlags:\n", synopsis)
		fset.PrintDefaults()
	}
	return fset.String("config", config.DefaultPath, "path to the settings file")
}

// loadConfig reads the settings, creating them only at the default
// location.
//
// A Host who typed a path has a file in mind; writing a fresh default
// one there would start a server that ignores everything they
// configured and says nothing about it. A missing file at the default
// location means a server nobody has configured yet, which is the case
// the auto-created file exists for.
func loadConfig(path string) (config.Config, error) {
	if path == config.DefaultPath {
		return config.LoadOrCreate(path)
	}
	return config.Load(path)
}

// logReachableURLs prints the addresses the rest of the table can use.
//
// Everyone but the Host joins over the LAN, and nothing happens until
// somebody reads an address out — which otherwise means going hunting
// through OS network settings for your own IP before you can start a
// game. Every candidate is printed rather than a chosen one: a machine
// with Wi-Fi, Ethernet and a VPN has three, and the Host is the only
// one who knows which network their players are on.
func logReachableURLs(addr string) {
	ifaces, err := lanurl.Interfaces()
	if err != nil {
		// Not worth failing a start over — the server is already up, and
		// the worst outcome is a Host who looks their address up the way
		// they always have.
		slog.Warn("longtable: could not read network interfaces", "error", err)
		return
	}

	candidates := lanurl.For(addr, ifaces)
	if len(candidates) == 0 {
		slog.Warn("longtable: no network address found to share — is this machine on a network?")
		return
	}

	for _, candidate := range candidates {
		if candidate.Interface == "" {
			slog.Info("longtable: reachable at", "url", candidate.URL)
			continue
		}
		slog.Info("longtable: reachable at", "url", candidate.URL, "interface", candidate.Interface)
	}
}

// openStore opens the database at dbPath and wraps it in a Store,
// returning a close func that shuts the connection down. Shared by
// serve, the room admin CLI, and set-banner/clear-banner.
func openStore(dbPath string) (*store.Store, func(), error) {
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}

	s, err := store.New(database)
	if err != nil {
		database.Close()
		return nil, nil, fmt.Errorf("open store: %w", err)
	}

	return s, func() { database.Close() }, nil
}
