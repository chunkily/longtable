// Command longtable is the self-hosted Longtable server: a single binary
// that serves the frontend, the API, and the real-time sync socket, and
// persists to an embedded SQLite database.
//
// Usage:
//
//	longtable [serve] [-addr :8080] [-db longtable.db] [-assets longtable-assets]
//	longtable room list [-db longtable.db]
//	longtable room reset-password [-db longtable.db] <slug>
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	assets "longtable"
	"longtable/internal/api"
	"longtable/internal/blobstore"
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
		slog.Error("longtable: fatal error", "error", err)
		os.Exit(1)
	}
}

func runServe(args []string) error {
	fset := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fset.String("addr", ":8080", "address to listen on")
	dbPath := fset.String("db", "longtable.db", "path to the SQLite database file")
	assetsDir := fset.String("assets", "longtable-assets", "directory for uploaded map/token images")
	// A Host's own announcement — "back up at 9", "new server address
	// next week". Shown to everyone on this server, in every room, and
	// dismissable by each of them; changing the text brings it back for
	// people who dismissed the last one. A flag rather than a screen
	// because a Host runs the server and needn't be at any table on it.
	banner := fset.String("banner", "", "a message shown to everyone on this server until they dismiss it")
	fset.Parse(args)

	return serve(*addr, *dbPath, *assetsDir, *banner)
}

func serve(addr, dbPath, assetsDir, banner string) error {
	s, closeDB, err := openStore(dbPath)
	if err != nil {
		return err
	}
	defer closeDB()

	blobs, err := blobstore.New(assetsDir)
	if err != nil {
		return fmt.Errorf("open asset storage: %w", err)
	}

	frontend, err := fs.Sub(assets.Dist, assets.DistDir)
	if err != nil {
		return fmt.Errorf("load embedded frontend: %w", err)
	}

	hub := ws.NewHub(s)
	router := api.NewRouter(s, hub, blobs, frontend, banner)

	slog.Info("longtable: listening", "addr", addr, "db", dbPath, "assets", assetsDir)
	logReachableURLs(addr)
	return http.ListenAndServe(addr, router)
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
