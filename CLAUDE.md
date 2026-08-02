# Longtable — working notes for Claude

Self-hosted virtual tabletop for D&D. One Go binary with the SvelteKit frontend embedded; the GM
runs it on their own machine and everyone else joins over LAN in a browser.

The README is deliberately thin — the pitch, how to run it, how to test it — because the shape of
this codebase still changes weekly and a README nobody re-reads goes stale silently. **The
architecture and current state live here instead**, and keeping them true is part of the work
(see [Keeping these docs current](#keeping-these-docs-current)).

## Layout

| Path | What lives there |
| --- | --- |
| `cmd/longtable/` | entrypoint: `serve` (default) plus a `room list` / `room reset-password` admin CLI |
| `internal/api/` | HTTP routes: create/join room, GM login, asset upload + serving + per-room library listing, health check, `GET /ws` upgrade, SPA fallback for the embedded frontend |
| `internal/ws/` | the real-time hub and the authority on room state — command/event protocol, permission checks, broadcast |
| `internal/store/` | SQLite schema and every typed query (rooms, participants, scenes, tokens, fog, drawings, chat). `store.go` holds the `CREATE TABLE`s and migrations |
| `internal/imageproc/` | decodes and re-encodes every upload to WebP. Read its doc comment before touching it — the studio-swing trap in there is easy to reintroduce |
| `internal/blobstore/` | re-encoded images on disk, addressed by content hash so identical uploads share a file |
| `internal/auth/` | session tokens and bcrypt password hashing. No accounts — identity in a room is a token in `localStorage` |
| `internal/dice/` | `/roll 2d6+3` expression parser |
| `internal/db/` | SQLite wiring (`modernc.org/sqlite`, no CGO) |
| `assets.go` (root) | `go:embed`s `web/build`. Has to be at the root — embed can't reach outside its own directory |
| `web/src/lib/api.ts`, `session.ts` | REST client; per-room session in `localStorage` |
| `web/src/lib/room.svelte.ts` | `RoomClient`: the WS protocol wrapped in Svelte 5 runes state |
| `web/src/lib/components/game-canvas.svelte` | the whole Konva map: layers, tools, rendering |
| `web/src/routes/r/[slug]/+page.svelte` | the room page — join form, toolbar, chat |
| `web/e2e/` | Playwright specs; several read canvas pixels because Konva has no DOM |
| `planning/` | backlog, user stories, ADRs (`decisions/`), role glossary |

Flow for anything that happens on the map: client sends a **command** over the socket → the hub
validates it, applies it through the store, and broadcasts an **event** to the room →
`RoomClient` folds the event into runes state → `game-canvas.svelte` re-renders. The Go side is
authoritative; the client never writes to the database.

## Where things stand

Working today: rooms with a GM password and player join, scenes built from an uploaded map or a
picked library asset and managed from a picker (switch, delete, swap the map under one),
tokens (GM creates, edits — name, art, size, visibility — and deletes, anyone drags, hidden ones
withheld from players; anyone can click one to select it, which rings it on the map and shows its
details above chat, with Edit and Delete beside them for a GM — the selection is local to that
browser, never synced),
fog the GM paints on and off a square at a time (plus reveal-all and reset for the whole scene),
drawings (freehand/line/rect/ellipse) with an eraser, and per-session undo/redo covering
drawing, erasing and token deletion,
pings, distance measuring, area-of-effect templates (circle/cone/line/cube, origin on a snap
mode and size in whole 5 ft steps),
chat with `/roll`, and a live list of who's connected (distinct from the room's roster of
everyone who has ever joined, which `state.sync` also carries). A dropped socket reconnects on
its own with backoff, and says so on screen while it's down. Every upload is decoded and
re-encoded to WebP and
joins the uploading room's library — content-addressed globally so identical uploads share one
file, but a room only ever sees what it added itself. All of it syncs live; everything but pings
and measurements persists.

Known gaps, which is also roughly the queue: no initiative tracker, no HP or conditions on a
token, no way to assign a token's owner (the roster is on the wire now, but nothing offers it as
a picker), fog has no
automatic vision from tokens, the asset library is an unfiltered grid with
no search, no prebuilt releases, no way for a Host to remove a moderated asset or cap upload
sizes per room.

`planning/backlog/` is the authority on all of this and goes into far more detail: `done/`
records what shipped and why, `in-progress/` and `open/` are the queue. Items cite paths and line
numbers, which is often the fastest orientation available — but they go stale, so verify before
relying on one.

## Commands

```bash
cd web && npm install && npm run build && cd .. && go build -tags nodynamic -o longtable ./cmd/longtable
```

| Task | Command |
| --- | --- |
| Go build/vet/test | `go build -tags nodynamic ./internal/... ./cmd/...`, same for `go vet` and `go test` |
| Frontend unit tests | `npm --prefix web run test` (vitest + jsdom) |
| Types | `npm --prefix web run check` (svelte-check) |
| Lint + format | `npm --prefix web run lint` (prettier check + eslint), `... run format` to fix |
| E2E | `cd web && npx playwright test` — builds the Go binary and starts both servers itself |

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
non-obvious line has a note on the constraint or failure that produced it — the `track()` helper
existing because `$effect` can't see reads after an `await`, `isCanonicalUUID` rejecting the
braced spelling so an echoed id stays byte-identical, the LIFO defer ordering in `ServeHTTP`.
When you change something with a reason behind it, leave the reason. When you find such a
comment, treat it as load-bearing: it's usually recording a bug someone already hit.

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
  Its `references/` hold the command/event table, the canvas layer table and the testing
  harnesses.
- `.claude/skills/longtable-backlog/` — how `planning/` works: picking an item up, moving it
  through the folders, writing the "What shipped" note.

## Keeping these docs current

Each file owns something, and duplication between them is what rots first:

| File | Owns |
| --- | --- |
| `README.md` | what Longtable is, how to run it, how to test it. Nothing that churns weekly |
| `CLAUDE.md` | layout, the command→event→state→render flow, current state, commands, house style |
| `.claude/skills/longtable-feature/references/ws-protocol.md` | every command and event, who may send it, what persists |
| `.claude/skills/longtable-feature/references/canvas.md` | Konva layer order and indices, the tool-handler contract |
| `.claude/skills/longtable-feature/references/testing.md` | the three test harnesses and their helpers |
| `planning/` | why a thing exists, what shipped, what's next |

Update them **in the same commit as the change**, not in a later sweep — a doc corrected a week
after the fact has already misled someone. The triggers, all cheap:

- Added a package, or moved something in the table above → the layout table here.
- Added or changed a WS command or event, or who's allowed to send one → the command table in
  `ws-protocol.md`.
- Added a Konva layer or a tool → the layer table in `canvas.md`, **and** the layer-order
  comments in `web/e2e/*.spec.ts`, which index layers by number.
- Added a test helper or changed how a suite runs → `testing.md`, and the README if the command
  a human types changed.
- Shipped a feature or closed a gap → "Where things stand" above, plus the backlog move and its
  "What shipped" note.
- Found one of these docs contradicting the code → fix it then, even mid-task. It's a two-line
  edit now and an hour of someone's confusion later. If the contradiction is in a `planning/done/`
  note, correct it in place rather than deleting the old text: the reasoning that turned out to be
  wrong is usually why the code changed.
