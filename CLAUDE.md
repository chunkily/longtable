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
| `web/src/routes/r/[slug]/assets/+page.svelte` | the assets page — the only way art enters a room's library, and the only way one leaves: tabbed by token/map, with name, credit, grid alignment, search |
| `web/e2e/` | Playwright specs; several read canvas pixels because Konva has no DOM |
| `planning/` | backlog, user stories, ADRs (`decisions/`), role glossary |

Flow for anything that happens on the map: client sends a **command** over the socket → the hub
validates it, applies it through the store, and broadcasts an **event** to the room →
`RoomClient` folds the event into runes state → `game-canvas.svelte` re-renders. The Go side is
authoritative; the client never writes to the database.

## Where things stand

Working today: rooms with a GM password and player join (the generated join slug is re-rolled if
it happens to spell something offensive), scenes built from an uploaded map or a
picked library asset and managed from a picker (switch, delete, swap the map under one),
tokens (GM creates and edits them with the same set of fields either way — name, art, size, owner,
visibility — and deletes, anyone drags, hidden ones
withheld from players; a token someone else moves slides to its new square rather than jumping;
anyone can click one to select it, which rings it on the map and shows its
details above chat, whose token it is included, with Edit beside them for a GM or for whoever owns
it and Delete for a GM alone — the
selection is local to that browser, never synced; each token also carries three numeric trackers,
labelled per token (hit points, armour class, a resource), and any number of condition tags — all
three slots shown in that details panel as large boxes whose values are typed straight into them,
a focused box floating a −/+ control that steps by 1 or by whatever it's told, plus a card when
the pointer rests on the token showing only the slots carrying a number; labels and conditions are
set in the edit dialog, and a GM may change all of it on any token while a Player may change the
trackers and conditions on one they own, which is the first thing ownership actually confers),
fog the GM paints on and off a square at a time (plus reveal-all and reset for the whole scene),
drawings (freehand/line/rect/ellipse) with an eraser, and per-session undo/redo covering
drawing, erasing, token deletion and token moves (an undo passes over a token someone else has
moved since, rather than dragging it back out from under them),
pings, distance measuring, area-of-effect templates (circle/cone/line/cube, origin on a snap
mode and size in whole 5 ft steps),
chat with `/roll`, and a live list of who's connected (distinct from the room's roster of
everyone who has ever joined, which `state.sync` also carries). A dropped socket reconnects on
its own with backoff, and says so on screen while it's down. Art enters a room only through the
assets page at `/r/{slug}/assets`. That page is tabbed by kind, Tokens and Maps, and the tab
governs all of it: what the library grid shows, and what anything added from there is filed as —
chosen before the file dialog opens rather than asked for afterwards. An upload is named
(defaulting to the filename minus its extension), credited, and — for a map — aligned to the grid
before it's added; if its dimensions disagree with the tab it was staged under, the card says so
and offers to move it, but never moves it on its own. Name, credit and kind are all editable
afterwards, and an asset can be removed from a room's library (the shared file survives, and
anything already using it keeps it). Token art is shown whole in a square tile, maps in a wide
crop; the library there and the pickers in the room share one searchable component, the pickers
open on the kind they're asking for without hiding the other, and the pickers only pick — their
link out to the assets page carries the open tab with it (`?kind=`), so art added after following
it is filed as the thing that was being looked for. Every
upload is decoded and re-encoded to WebP, with any grid offset padded into the pixels on the way
through, and joins the uploading room's library — content-addressed globally so identical uploads
share one file, but a room only ever sees what it added itself, under its own name and credit.
All of it syncs live; everything but pings and measurements persists.

Known gaps, which is also roughly the queue: no initiative tracker, ownership governs a token's
trackers and conditions but nothing else (anyone can still move anyone's token),
fog has no automatic vision from tokens, no prebuilt releases, no way for a Host to
remove a moderated asset server-wide or cap upload sizes per room (a room removing something from
its own library is a different, smaller thing, and does exist).

One of those bites specifically when the app is used the way it's meant to be — a GM hosting and
everyone else on `http://192.168.x.x:8080` from their own device — and doesn't show up on
localhost or in the test suites, so it's worth knowing before anyone debugs it from scratch:
**the map can't be zoomed on a touch device**, since the only thing that scales the stage is a
mouse wheel. See `planning/backlog/pinch-zoom-touch-devices.md`.

Its twin is fixed. Ids used to be minted with `crypto.randomUUID`, which browsers expose only in a
secure context, so drawing and pings threw for every client on a LAN address. They now go through
`randomId()` in `web/src/lib/random-id.ts`, which falls back to `crypto.getRandomValues` — read its
doc comment before minting an id anywhere else, and **never call `crypto.randomUUID` directly**.
Anything else gated on a secure context (`navigator.clipboard` is the likely next one, for a "copy
the join link" button) has the same trap waiting.

`planning/backlog/` is the authority on all of this and goes into far more detail: `status: done`
items record what shipped and why, `status: open` ones are the queue. Items cite paths and line
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
- `.claude/skills/longtable-backlog/` — how `planning/` works: picking an item up, flipping its
  `status:` field when it ships, writing the "What shipped" note, and flipping a user story's
  `status:` field once its criteria are verified against the code.

## Keeping these docs current

Each file owns something, and duplication between them is what rots first:

| File | Owns |
| --- | --- |
| `README.md` | what Longtable is, how to run it, how to test it. Nothing that churns weekly |
| `CLAUDE.md` | layout, the command→event→state→render flow, current state, commands, house style |
| `.claude/skills/longtable-feature/references/ws-protocol.md` | every command and event, who may send it, what persists |
| `.claude/skills/longtable-feature/references/canvas.md` | Konva layer order and indices, the tool-handler contract |
| `.claude/skills/longtable-feature/references/testing.md` | the three test harnesses and their helpers |
| `.claude/skills/longtable-backlog/SKILL.md` | how `planning/` itself works — the status fields, item and story format, what finishing something entails |
| `planning/` | why a thing exists, what shipped, what's next |

`planning/backlog/README.md` and `planning/user-stories/README.md` restate those same rules for
someone browsing the folder without the skill loaded. Three files saying one thing is exactly the
duplication this table exists to catch, so treat the skill as the authority and the two READMEs as
its summary — when the convention changes, all three move together or the odd one out starts
lying.

Update them **in the same commit as the change**, not in a later sweep — a doc corrected a week
after the fact has already misled someone. The triggers, all cheap:

- Added a package, or moved something in the table above → the layout table here.
- Added or changed a WS command or event, or who's allowed to send one → the command table in
  `ws-protocol.md`.
- Added a Konva layer or a tool → the layer table in `canvas.md`, **and** the layer-order
  comments in `web/e2e/*.spec.ts`, which index layers by number.
- Added a test helper or changed how a suite runs → `testing.md`, and the README if the command
  a human types changed.
- Shipped a feature or closed a gap → "Where things stand" above, plus flipping the backlog item's
  `status:`, its "What shipped" note, and flipping the linked user story's `status:` to `done`
  once every acceptance criterion actually holds (see `longtable-backlog`).
- Changed how `planning/` itself works — status values, file format, where a thing lives → the
  `longtable-backlog` skill **and** both `planning/` READMEs, in one commit.
- Found one of these docs contradicting the code → fix it then, even mid-task. It's a two-line
  edit now and an hour of someone's confusion later. If the contradiction is in a `planning/done/`
  note, correct it in place rather than deleting the old text: the reasoning that turned out to be
  wrong is usually why the code changed.
