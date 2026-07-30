# Longtable — working notes for Claude

Self-hosted virtual tabletop for D&D. One Go binary with the SvelteKit frontend embedded; the GM
runs it on their own machine and everyone else joins over LAN in a browser. See
[README.md](README.md) for the pitch, the architecture and the test commands.

For what's actually built and what's next, `planning/backlog/` beats the README every time:
`done/` records what shipped and the reasoning behind it, `in-progress/` and `open/` are the
queue. Backlog items cite file paths and line numbers, which is often the fastest orientation
available — but they go stale, so verify before relying on one.

## Layout

| Path | What lives there |
| --- | --- |
| `cmd/longtable/` | entrypoint: `serve` (default) plus a `room list` / `room reset-password` admin CLI |
| `internal/api/` | HTTP routes; REST for rooms/assets, SPA fallback for the embedded frontend, `GET /ws` upgrade |
| `internal/ws/` | the real-time hub — command/event protocol, permission checks, broadcast |
| `internal/store/` | SQLite schema and every typed query. `store.go` holds `CREATE TABLE`s and migrations |
| `internal/blobstore/` | uploaded images on disk, content-addressed |
| `internal/dice/` | `/roll 2d6+3` expression parser |
| `assets.go` (root) | `go:embed`s `web/build`. Has to be at the root — embed can't reach outside its own directory |
| `web/src/lib/room.svelte.ts` | `RoomClient`: the WS protocol wrapped in Svelte 5 runes state |
| `web/src/lib/components/game-canvas.svelte` | the whole Konva map: layers, tools, rendering |
| `web/e2e/` | Playwright specs; several read canvas pixels because Konva has no DOM |
| `planning/` | backlog, user stories, ADRs (`decisions/`), role glossary |

Flow for anything that happens on the map: client sends a **command** over the socket → the hub
validates it, applies it through the store, and broadcasts an **event** to the room →
`RoomClient` folds the event into runes state → `game-canvas.svelte` re-renders. The Go side is
authoritative; the client never writes to the database.

## Commands

```bash
cd web && npm install && npm run build && cd .. && go build -o longtable ./cmd/longtable
```

| Task | Command |
| --- | --- |
| Go build/vet/test | `go build ./internal/... ./cmd/...`, `go vet ./internal/... ./cmd/...`, `go test ./internal/... ./cmd/...` |
| Frontend unit tests | `npm test` (in `web/`, vitest + jsdom) |
| Types | `npm run check` (svelte-check) |
| Lint + format | `npm run lint` (prettier check + eslint), `npm run format` to fix |
| E2E | `npx playwright test` (in `web/`) — builds the Go binary and starts both servers itself |

Scope the Go commands to `./internal/... ./cmd/...`, not `./...`: the repo root also contains
`web/node_modules`, which the go tool would walk looking for packages. CI (`.github/workflows/ci.yml`)
runs all of the above, with `-race` on the Go tests — `-race` needs cgo, so it can't run on this
Windows box, but it does run on Linux CI.

## Before touching a shared port

The e2e harness binds **:8080** (Go backend) and **:5173** (vite), and other Claude sessions
work in this same checkout. A process already on either port may well be in use — ask before
stopping anything. `npx playwright test` also writes to the shared `web/.e2e-data/longtable.db`,
so a run leaves test rooms in whatever data another session is looking at.

## House style

The thing that most distinguishes this codebase: **comments explain why, not what.** Nearly every
non-obvious line has a note on the constraint or failure that produced it — `renderDrawings`
using `draw()` instead of `batchDraw()`, `GridOffset*` being dead, the LIFO defer ordering in
`ServeHTTP`. When you change something with a reason behind it, leave the reason. When you find
such a comment, treat it as load-bearing: it's usually recording a bug someone already hit.

Others worth matching:

- Prose in comments and planning docs uses em dashes and reads like sentences, not telegraphese.
- Go errors: `slog.Error` with the internal detail, then a short human message to the client.
  Never leak the internal error over the socket.
- Svelte 5 runes only (`$state`/`$derived`/`$effect`), enforced in `vite.config.ts`. Plain
  `Map`/`Set` for imperative bookkeeping nothing reactive reads — with the eslint-disable comment
  the linter wants, and a note saying why it isn't a `SvelteMap`.
- Tests assert behaviour and name the reason in the test name or a comment above it. `go test`
  helpers take `t` and call `t.Helper()`.

## Skills in this repo

- `.claude/skills/longtable-feature/` — adding or changing a room feature end to end (protocol,
  client state, canvas, tests). Read it before touching `internal/ws/` or `game-canvas.svelte`.
- `.claude/skills/longtable-backlog/` — how `planning/` works: picking an item up, moving it
  through the folders, writing the "What shipped" note.
