// Command longtable is the self-hosted Longtable server: a single binary
// that serves the frontend, the API, and the real-time sync socket, and
// persists to an embedded SQLite database.
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
	"longtable/internal/db"
	"longtable/internal/ws"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	dbPath := flag.String("db", "longtable.db", "path to the SQLite database file")
	flag.Parse()

	if err := run(*addr, *dbPath); err != nil {
		slog.Error("longtable: fatal error", "error", err)
		os.Exit(1)
	}
}

func run(addr, dbPath string) error {
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	frontend, err := fs.Sub(assets.Dist, assets.DistDir)
	if err != nil {
		return fmt.Errorf("load embedded frontend: %w", err)
	}

	hub := ws.NewHub()
	router := api.NewRouter(hub, database, frontend)

	slog.Info("longtable: listening", "addr", addr, "db", dbPath)
	return http.ListenAndServe(addr, router)
}
