# Longtable

Open source virtual tabletop (VTT) for Dungeons and Dragons.

Self-hosted: the GM runs a single Go binary (with the frontend and an
embedded SQLite database baked in) and players connect over the browser.
No external services, no separate database to install.

## Architecture

- **`web/`** — SvelteKit frontend (TypeScript, Konva for the map/token
  canvas), built as a static SPA (`npm run build` → `web/build`).
- **`cmd/longtable/`** — entrypoint for the Go binary: opens the SQLite
  database, wires up the router, starts the HTTP server.
- **`internal/api/`** — HTTP routes: serves the embedded frontend (with
  SPA fallback for client-side routes), the health check, and the
  WebSocket upgrade.
- **`internal/ws/`** — the real-time sync hub. Currently a broadcast
  stub; the actual map/token/fog protocol isn't designed yet.
- **`internal/db/`** — SQLite wiring (`modernc.org/sqlite`, no CGO).
  Schema isn't designed yet.
- **`assets.go`** (repo root) — `go:embed`s `web/build` into the binary.
  Go embed directives can't reach outside their own directory, so this
  has to live at the root, as a sibling of `web/`, rather than under
  `internal/`.

## v1 scope

Core tabletop only: map upload, token placement/movement, fog of war, a
basic dice roller, and real-time sync between the GM and players. No
character sheets or rules automation yet.

## Running locally

```bash
cd web && npm install && npm run build && cd ..
go build -o longtable ./cmd/longtable
./longtable
```

Serves on `:8080` by default (`-addr` and `-db` flags to override).

## Status

Early scaffolding. The server boots, serves the frontend with correct SPA
routing, and has a working (empty) SQLite connection and a WebSocket
broadcast stub — no game logic yet. Frontend is the default SvelteKit
starter page, not yet a tabletop UI.
