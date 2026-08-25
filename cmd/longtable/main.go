// Command longtable is the self-hosted Longtable server: a single binary
// that serves the frontend, the API, and the real-time sync socket, and
// persists to an embedded SQLite database.
//
// Usage:
//
//	longtable [serve] [-db longtable.db] [-addr :8080]
//	longtable config list|get|set|unset|export|import [-db longtable.db]
//	longtable admin [-db longtable.db] [-new]
//	longtable room list [-db longtable.db]
//	longtable room reset-password [-db longtable.db] <room-code>
//	longtable set-banner [-db longtable.db] <message>
//	longtable clear-banner [-db longtable.db]
//
// `longtable -h` prints the same list.
//
// **Settings live in the database**, not in a file — see ADR-0009 and
// internal/config, which is the registry of what a setting is rather
// than a reader of anything. `-db` says which database, and it is the
// only way to say so: two ways of naming one is how a Host ends up
// administering a file the running server has never opened.
//
// `-addr` is the one setting that is also a flag, and it exists for
// recovery. A stored listen address that nothing can reach would
// otherwise lock a Host out of the admin screen they would fix it from,
// so a flag beats the stored value.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	assets "longtable"
	"longtable/internal/admin"
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
// "what can this do?", it answers with a single -db line and no
// mention that `room` or `set-banner` exist at all.
//
// Each command's arguments are shown next to it, because the flag
// package can't: `longtable set-banner -h` lists -db and stops,
// which reads as "takes no arguments" for the two commands that need
// one.
const usage = `Longtable is a self-hosted virtual tabletop for D&D.

Usage:
  longtable [command] [-db longtable.db]

With no command, Longtable starts the server.

Commands:
  serve                            Start the server.
  config list                      Show every setting, and what each one does.
  config set <key> <value>         Change a setting.
  admin                            Print the link to the admin page.
  room list                        List every room on this server.
  room reset-password <room-code>  Set a new GM password for a room.
  set-banner <message>             Show a message across the top of every page.
  clear-banner                     Take that message down.

Flags:
  -db <path>                       The database to use. Default: longtable.db
  -addr <host:port>                Listen here instead of the stored address.

Run "longtable config -h" for the rest of the settings commands.
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
// serves, and so does `longtable -db other.db`, but `longtable serv`
// must not.
type route string

const (
	routeHelp        route = "help"
	routeServe       route = "serve"
	routeRoom        route = "room"
	routeSetBanner   route = "set-banner"
	routeClearBanner route = "clear-banner"
	routeConfig      route = "config"
	routeAdmin       route = "admin"
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
	case args[0] == "config":
		return routeConfig
	case args[0] == "admin":
		return routeAdmin
	case strings.HasPrefix(args[0], "-"):
		// A flag with no command in front of it belongs to serve, which is
		// what `longtable -db other.db` means.
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
	case routeConfig:
		err = runConfigCommand(args[1:])
	case routeAdmin:
		err = runAdmin(args[1:])
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
	dbPath := addDBFlag(fset, "longtable [serve] [-db path] [-addr host:port]")
	// Empty rather than defaulted, so "not given" is distinguishable from
	// "given the same value the database holds" — only the first should
	// leave the stored setting in charge.
	addrOverride := fset.String("addr", "", "listen here instead of the stored address")
	fset.Parse(args)

	s, closeDB, err := openStore(*dbPath)
	if err != nil {
		return err
	}
	defer closeDB()

	cfg, err := loadSettings(s)
	if err != nil {
		return err
	}
	if *addrOverride != "" {
		cfg.Addr = *addrOverride
	}

	return serve(s, cfg, *dbPath)
}

func serve(s *store.Store, cfg config.Config, dbPath string) error {
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

	// Minted here so the URL can be printed, but the admin surface reads
	// it per request from the store — that is what makes `longtable admin
	// -new` take effect on a running server rather than at the next
	// restart. Not minted at all when the admin page is off: a secret
	// nothing can use is a secret not worth creating.
	token := ""
	if cfg.AdminAddr != "" {
		token, err = s.AdminToken()
		if err != nil {
			return fmt.Errorf("read admin token: %w", err)
		}
	}

	slog.Info("longtable: listening", "addr", cfg.Addr, "db", dbPath, "assets", cfg.Assets)
	logReachableURLs(cfg.Addr)
	if cfg.AdminAddr == "" {
		// Said rather than left silent. A Host who turned it off months
		// ago and has forgotten needs the line more than one who did it
		// this morning, and silence where a URL used to be reads as a
		// fault.
		slog.Info("longtable: admin page is off", "turn_it_on_with", "longtable config unset admin_addr")
	} else {
		logAdminURL(cfg.AdminAddr, token)
	}

	// Two listeners, and the server is only up when both are: a Host who
	// cannot reach the admin page has no way to fix the setting that
	// would let them, so failing to bind it is a failure to start rather
	// than something to log and carry on past.
	//
	// Whichever falls over first ends the process. Neither returns unless
	// something has gone wrong.
	failed := make(chan error, 2)
	if cfg.AdminAddr != "" {
		go func() {
			// The game's URL goes with it so the admin page can point back at
			// Longtable itself. Two addresses is two things to confuse, and the
			// Host who lands on the wrong one should find the way over rather
			// than a dead end.
			handler := admin.New(s.AdminToken, cfg.AdminAddr, browsableURL(cfg.Addr))
			failed <- fmt.Errorf("admin listener: %w", http.ListenAndServe(cfg.AdminAddr, handler))
		}()
	}
	go func() {
		failed <- http.ListenAndServe(cfg.Addr, router)
	}()
	return <-failed
}

// logAdminURL prints the way in to the admin page.
//
// The most important line on the screen for a first-time Host, and the
// only place the token is shown until they ask for it again with
// `longtable admin` — which they will, because a console scrolls and on
// Windows it closes with the process.
//
// It says "admin page" rather than anything shorter because there are
// now two addresses on this screen and only one of them is Longtable
// itself. This line read "set up at" first, which a Host reasonably took
// to mean "the app is set up at this URL", and they went to the admin
// port looking for their rooms.
func logAdminURL(adminAddr, token string) {
	slog.Info("longtable: admin page at", "url", adminURL(adminAddr, token))
}

// adminURL is the address and the token together, ready to be opened.
func adminURL(adminAddr, token string) string {
	return fmt.Sprintf("%s/?token=%s", browsableURL(adminAddr), token)
}

// browsableURL turns a listen address into one that can be typed into a
// browser on this machine.
//
// A bare ":8080" means every address this machine has, which is not
// something a browser can be given — "http://:8080" goes nowhere. From
// the Host's own seat the loopback address is the true one.
func browsableURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// defaultDatabase is where the server looks when nobody says otherwise:
// the working directory, beside the asset directory it also defaults to,
// so everything one server owns sits in one folder the Host chose.
//
// Here rather than in internal/config because it is not a setting — it
// is the one thing that cannot be, since nothing could read it.
const defaultDatabase = "longtable.db"

// addDBFlag registers the one flag every subcommand has: settings live
// in the database, and this says which database.
//
// It is the only way to name it. Two ways is how a Host ends up
// resetting a password, or changing a setting, in a file the running
// server has never opened — which is why the config file that used to
// name it took the same care.
//
// synopsis is the command's own usage line, and it is here rather than
// left to the flag package because that prints the flags and nothing
// else — a command with a required argument comes out looking like a
// command with none. Passing it in also means the same string can be
// the error text when the argument is missing, so the two can't
// disagree about what the command takes.
func addDBFlag(fset *flag.FlagSet, synopsis string) *string {
	fset.Usage = func() {
		fmt.Fprintf(fset.Output(), "Usage: %s\n\nFlags:\n", synopsis)
		fset.PrintDefaults()
	}
	return fset.String("db", defaultDatabase, "path to the database file")
}

// loadSettings reads what a Host has configured and resolves it against
// the registry's defaults.
//
// Validated here as well as on the way in. The write path refuses a bad
// value while somebody is present to be told, which is the point of
// settings living in a database — but a database can also be edited by
// hand, and a server should say what is wrong rather than start on
// something incoherent.
func loadSettings(s *store.Store) (config.Config, error) {
	values, err := s.Settings()
	if err != nil {
		return config.Config{}, fmt.Errorf("read settings: %w", err)
	}
	cfg, err := config.Resolve(values)
	if err != nil {
		return config.Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return config.Config{}, err
	}
	return cfg, nil
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
