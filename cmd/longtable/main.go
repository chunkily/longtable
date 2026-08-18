// Command longtable is the self-hosted Longtable server: a single binary
// that serves the frontend, the API, and the real-time sync socket, and
// persists to an embedded SQLite database.
//
// Usage:
//
//	longtable [serve] [-config longtable.toml]
//	longtable room list [-config longtable.toml]
//	longtable room reset-password [-config longtable.toml] <room-code>
//
// Every setting lives in the config file, which the server writes for
// itself the first time it runs. `-config` is the only flag there is:
// see internal/config for why.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
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

func main() {
	args := os.Args[1:]

	var err error
	if len(args) > 0 && args[0] == "room" {
		err = runRoomCommand(args[1:])
	} else {
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
	configPath := addConfigFlag(fset)
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
	router := api.NewRouter(s, hub, blobs, frontend, cfg.Banner)

	slog.Info("longtable: listening", "addr", cfg.Addr, "db", cfg.Database, "assets", cfg.Assets)
	logReachableURLs(cfg.Addr)
	return http.ListenAndServe(cfg.Addr, router)
}

// addConfigFlag registers the one flag every subcommand has. There are
// no others: settings live in the file, and this says which file.
func addConfigFlag(fset *flag.FlagSet) *string {
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
// returning a close func that shuts the connection down. Shared by both
// the serve command and the room admin CLI.
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
