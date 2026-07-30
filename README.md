# Longtable

A simple virtual tabletop (VTT) for playing Dungeons and Dragons online — for
hobbyist groups who want a way to run their game digitally without paying for a
subscription or software licenses using hardware they already own.

Longtable runs as a single program that one person in the group (usually the GM)
downloads and starts up on their own computer; everyone else just opens it in
their web browser to join.

## Architecture

- **`web/`** — SvelteKit frontend (TypeScript, Konva for the map/token canvas),
  built as a static SPA (`npm run build` → `web/build`).
- **`cmd/longtable/`** — entrypoint for the Go binary: opens the SQLite
  database, wires up the router, starts the HTTP server. Also a small `room`
  admin CLI (`room list`, `room reset-password`).
- **`internal/api/`** — HTTP routes: creating and joining rooms, GM login, asset
  upload/serving, the health check, the WebSocket upgrade, and the embedded
  frontend with SPA fallback for client-side routes.
- **`internal/ws/`** — the real-time sync hub, and the authority on room state.
  Clients send commands (move this token, erase this drawing); the hub
  validates and applies them through the store, then broadcasts the resulting
  event to everyone in the room.
- **`internal/store/`** — the SQLite schema and every typed query: rooms,
  participants, scenes, tokens, fog, drawings, chat.
- **`internal/blobstore/`** — uploaded images on disk, addressed by the hash of
  their content, so identical uploads share one file.
- **`internal/auth/`** — session tokens and bcrypt password hashing. There are
  no accounts: a browser's identity in a room is a token in `localStorage`.
- **`internal/dice/`** — the `/roll 2d6+3` expression parser.
- **`internal/db/`** — SQLite wiring (`modernc.org/sqlite`, no CGO).
- **`assets.go`** (repo root) — `go:embed`s `web/build` into the binary. Go
  embed directives can't reach outside their own directory, so this has to live
  at the root, as a sibling of `web/`, rather than under `internal/`.

## v1 scope

Core tabletop only: map upload, token placement/movement, fog of war, a basic
dice roller, and real-time sync between the GM and players. No character sheets
or rules automation yet.

## Running locally

```bash
cd web && npm install && npm run build && cd ..
go build -o longtable ./cmd/longtable
./longtable
```

Serves on `:8080` by default (`-addr` and `-db` flags to override).

## Running the tests

Three suites. All of these run from the repo root, and CI runs all of them on
every push and pull request.

**Go** — the sync hub, the store and its migrations, the REST API, the dice
parser:

```bash
go test ./internal/... ./cmd/...
```

Scope it to `./internal/... ./cmd/...` rather than `./...`: the repo root also
contains `web/node_modules`, which the go tool would otherwise walk looking for
packages. CI additionally runs with `-race`, which needs cgo and so won't work
against the CGO-free SQLite driver on a default Windows setup — Linux CI covers
it.

**Frontend unit tests** — the WebSocket client's state handling, and the pure
geometry modules (grid distance, drawing hit-testing). Vitest, no browser:

```bash
npm --prefix web run test
```

**End-to-end** — Playwright, driving real browsers against a real server. It
builds the Go binary and starts both the backend and the dev server itself, so
there's nothing to start first:

```bash
cd web && npx playwright test
```

The e2e suite needs ports **8080** and **5173** free, and writes to its own
scratch database under `web/.e2e-data/` (never your local `longtable.db`). First
run needs the browser: `npx playwright install chromium`. On Windows the first
run may also raise a firewall prompt for the freshly built server binary — it's
built to a fixed path, so approving it once is enough.

Several specs work by reading pixels off the Konva canvas, since the map has no
DOM to assert against. To run just one while working on it:

```bash
cd web && npx playwright test measure.spec.ts
```

Two more checks that aren't tests but will fail CI:

```bash
npm --prefix web run check && npm --prefix web run lint
```

`check` is svelte-check (types), `lint` is prettier plus eslint;
`npm --prefix web run format` fixes formatting.

## Documentation

See [`docs/`](docs/) for guides on hosting and configuring a server.

## Status

Playable, and rough in places. A GM can create a room, start a scene from an
uploaded map, place tokens and drag them around, and paint fog away cell by
cell; everyone in the room shares a chat log with dice rolls, can draw on the
map (freehand, lines, rectangles, ellipses) in a few colours, erase, undo and
redo their own work, ping a spot, and measure distances in feet. All of it syncs
live between browsers, and everything except the pings and measurements — which
are meant to be momentary — survives a reload.

The biggest gaps, roughly in the order they hurt: there's no way to switch back
to a scene once another is active, no initiative tracker, no way to inspect or
edit a token after creating it (so no HP or conditions), fog only reveals and
never hides again, and a dropped WebSocket doesn't reconnect on its own.
Uploaded images are also stored exactly as received — the decode-and-re-encode
pass that [ADR-0005](planning/decisions/0005-webp-reencoding-library.md) settled
on hasn't been built yet. There are no prebuilt binaries, so hosting means
building from source.

[`planning/backlog/`](planning/backlog/) is the live picture: `done/` records
what shipped and why, `in-progress/` and `open/` what's next.
