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
| `cmd/longtable/` | entrypoint: `serve` (default) plus a `room list` / `room reset-password` admin CLI. Every subcommand takes `-config` and nothing else — settings live in the file, including the database path the room CLI uses to find the same database the server has open |
| `internal/api/` | HTTP routes: create/join room, GM login, asset upload + serving + per-room library listing, health check, the Host's banner (`GET /api/notice`), `GET /ws` upgrade, SPA fallback for the embedded frontend |
| `internal/ws/` | the real-time hub and the authority on room state — command/event protocol, permission checks, broadcast |
| `internal/store/` | SQLite schema and every typed query (rooms, participants, scenes, tokens, fog, drawings, chat). `store.go` holds the `CREATE TABLE`s, plus `addMissingColumns` — the only way a column reaches a database that already exists, since `CREATE TABLE IF NOT EXISTS` won't. **No `CHECK` constraints**: a value set that can grow has to live in Go, and `TestSchema_HasNoCheckConstraints` enforces it. `fog.go` stores the *hidden* cells packed 32 to an integer — read its doc comment before touching fog anywhere |
| `internal/imageproc/` | decodes and re-encodes every upload to WebP. Read its doc comment before touching it — the studio-swing trap in there is easy to reintroduce |
| `internal/blobstore/` | re-encoded images on disk, addressed by content hash so identical uploads share a file |
| `internal/auth/` | session tokens and bcrypt password hashing. No accounts — identity in a room is a *seat* (a `participant` row), and a device proves it holds one with a `session` token in `localStorage`. See [ADR-0008](planning/decisions/0008-seats-and-sessions.md) |
| `internal/config/` | the Host's `longtable.toml`: every setting there is, the defaults, and the commented file the server writes for itself when there isn't one. Written from a template rather than marshalled, because comments are the whole reason it's TOML ([ADR-0006](planning/decisions/0006-config-file-format.md)) — and written once, never rewritten |
| `internal/dice/` | `/roll 2d6+3` expression parser |
| `internal/lanurl/` | which of the machine's addresses a Host can hand to their players, printed at startup. Takes the interface list as an argument, so the rules are testable on machines nobody has |
| `internal/db/` | SQLite wiring (`modernc.org/sqlite`, no CGO) |
| `assets.go` (root) | `go:embed`s `web/build`. Has to be at the root — embed can't reach outside its own directory |
| `web/src/lib/api.ts`, `session.ts` | REST client; per-room session in `localStorage` |
| `web/src/lib/room-code.ts` | validates six typed characters as a room code. Codes only — a pasted link is refused, and its doc comment says why |
| `web/src/lib/room.svelte.ts` | `RoomClient`: the WS protocol wrapped in Svelte 5 runes state |
| `web/src/lib/token-drag.ts` | where a dragged token lands and how far that is — the one answer behind both the preview shown mid-drag and the square `dragend` actually sends |
| `web/src/lib/token-fields.ts` | what a `token.update` carries and whether two of them are the same — the one answer behind the dialog's "anything typed?", `updateToken`'s no-op guard and undo's "still how I left it?" |
| `web/src/lib/components/game-canvas.svelte` | the whole Konva map: layers, tools, rendering |
| `web/src/lib/tool-family.ts` | the `Tool` union, and the rules grouping it into the toolbar's five families |
| `web/src/lib/components/map-toolbar.svelte`, `tool-strip.svelte` | the floating tool row, and the active family's contextual strip |
| `web/src/lib/stroke-colors.ts`, `components/stroke-color-picker.svelte` | the eight colours a drawing can be — light-map hues above, their bright counterparts for dark maps below — and the only place their hex lives, including the default every browser starts on; plus the one strip button they open from |
| `web/src/lib/components/room-menu.svelte` | the menu behind the side panel's third icon: Scenes, Assets, Manage room, Leave room |
| `web/src/lib/identity-color.ts` | the sixteen colours a seat can be, and the only place their hex lives. A seat stores the *key*; `store.IdentityColors` in Go is the same list and validates it, since the value reaches a `style` attribute. `TestIdentityColors_MatchTheClientPalette` fails if the two drift |
| `web/src/lib/components/ui/popover/` | the bits-ui popover, wrapped the way `ui/dialog` is. Every popup in the room is on it — the room menu and the draw strip's stroke width — and anything new that pops up over the map should be too, for the focus handling rather than the placement |
| `web/src/lib/host-notice.svelte.ts`, `components/host-notice.svelte` | the Host's `banner` message: fetched once, dismissable, and the height everything else moves down by |
| `web/src/lib/components/theme-toggle.svelte` | System/Light/Dark as three icon buttons, in two shapes: a labelled row for the room menu and a floating pill for the home page's corner. The scheme itself is `mode-watcher`, wired up in `+layout.svelte`, plus the boot script in `app.html` that beats the flash of light |
| `web/src/lib/components/initiative-panel.svelte` | the turn order in the rail's second panel — one component for both roles, with the GM's controls left off for everyone else |
| `internal/ws/initiative.go` | the tracker's six commands and its one event, split out of `hub.go` |
| `web/src/routes/r/[slug]/+page.svelte` | the room page — join form, then the full-bleed shell: map, floating toolbar, side rail (or bottom sheet) |
| `web/e2e/fixtures/` | everything the specs are built on, so `e2e/` itself is just the tests: the `table` fixture (a room, a scene, a GM, and teardown that survives a failure), the canvas helpers, the room-chrome helpers (including `createRoom`, which every spec goes through), and the upload images. Its README is the starting point for a new spec |
| `web/src/routes/r/[slug]/assets/+page.svelte` | the assets page — the only way art enters a room's library, and the only way one leaves: tabbed by token/map, with name, credit, grid alignment, search |
| `web/e2e/` | Playwright specs; several read canvas pixels because Konva has no DOM |
| `planning/` | backlog, user stories, ADRs (`decisions/`), role glossary |

Flow for anything that happens on the map: client sends a **command** over the socket → the hub
validates it, applies it through the store, and broadcasts an **event** to the room →
`RoomClient` folds the event into runes state → `game-canvas.svelte` re-renders. The Go side is
authoritative; the client never writes to the database.

## Where things stand

Working today: rooms with a GM password and player join (the generated join slug is re-rolled if
it happens to spell something offensive). **The join screen asks one question at a time**, and the
first is which side of the screen you're on: `I'm the GM` leads to a name and the room password,
`Player` leads to the room's seats with `I'm new here` as a dashed slot at the foot of that list,
which is what leads to a name box. Every step but the first can go back. **Identity in a room is a
seat, not a browser**: a
device with no stored session gets the room's seats and takes one, which brings back the tokens
that seat owns and its name — so a cleared browser or a borrowed laptop costs a session rather
than an identity, and one person on a phone and a laptop is two sessions and one entry in the
roster. Claiming is open, with no password or approval; the GM's seat is the exception and goes
through the room password, which also means a second GM login reuses that seat instead of growing
the roster. A GM can add a seat before anyone arrives and remove a finished one from `Manage
room`, and **set a new room password** from the same dialog — no current password asked for, since
the session already proves the seat, and nobody is signed out by the change, including whoever
made it. The same dialog is where a room **ends**: `Delete room` arms and then fires,
takes the scenes, tokens, chat and seats with it, and leaves the uploaded images alone — they are
shared with any room that uploaded the same bytes. Anyone sitting in it is told over the socket
(`room.deleted`) and sent home rather than left on a socket that stops answering. `longtable room reset-password` stays the Host's path for a GM who can't get in at all;
leaving a room ends that device's session and leaves the seat behind to come back to.
**Rooms are not listed anywhere** — there is no server endpoint that enumerates
them, and `longtable room list` is the only way to see them all, which is the Host's job and needs
the database file. The way into one is its **room code**: six characters, and the word used for it
everywhere a person can read one — both steps of the home page, the pre-join screen, the README,
the hosting guide, and that CLI's `CODE NAME CREATED` header. `slug` survives as the route
parameter, the column and the Go identifiers, because nobody is ever asked to say a URL shape out
loud. **The home page asks one question at a time** too: a browser holding sessions sees those
rooms listed, newest first, and every browser gets two large buttons under it — `Join a room`,
which opens one large six-character box (codes only — a link is meant to be followed, not pasted
in there, and a bad code is answered in danger text under the box rather than a toast), and
`Create a room`. A code that's well formed but has no room behind it is answered on arrival: the
pre-join screen is replaced by `Room not found!`, off the seat endpoint's 404 — only a 404,
since a blip says nothing about whether the room exists. A browser with
no rooms gets no list at all rather than an empty one, since the two buttons are what it came for.
Scenes built
from an uploaded map or a
picked library asset and managed from one dialog (make, switch, delete, swap the map under one),
tokens (**anyone creates them**, from the same `New token` icon on the toolbar and up to twenty at
a time — a batch is numbered `Monkey 1`…`Monkey 8` and spreads over free squares rather than
stacking, and each one is its own undo. A GM's dialog carries name, art, size, owner and
visibility; a Player's is the same minus the last two, because a token they make is theirs and
hiding one is a GM power. A GM edits and deletes anything, an owner edits and deletes their own,
anyone drags — unless the GM has set `Manage room`'s **Moving tokens** to `Only the owner`, which
holds every Player to their own tokens and leaves the GM able to move anything; a token someone
else moves slides to its new square rather than jumping; **dragging one shows how far it's going**
— a translucent ghost stays on the square it was picked up from, a dashed line runs to the square
it would land on, and the distance in feet floats above it, counted in whole squares by the same
diagonal rule as the ruler and reading off the square it will actually snap to rather than the
cursor. That overlay is local to the dragger's own browser and never broadcast, like the selection
ring;
anyone can click one to select it, which rings it on the map and shows its
details above chat, whose token it is included, with a pen and a bin beside them for a GM or for
whoever owns it — the
selection is local to that browser, never synced. **Touching a token also brings it to the top of
the stack** — a click, the start of a drag, or its entry in the initiative tracker — so an
overlapping pile can be worked through rather than leaving whatever was made last permanently on
top. That order is this screen's alone too: never sent, never stored, and gone on a reload, which
puts everything back in creation order. Each token also carries three numeric trackers,
labelled per token (hit points, armour class, a resource), and any number of condition tags — all
three slots shown in that details panel as large boxes whose values are typed straight into them,
a focused box floating a −/+ control that steps by 1 or by whatever it's told, plus a card when
the pointer rests on the token showing only the slots carrying a number; labels and conditions are
set in the edit dialog — which **stages** its changes, unlike the panel: `Save changes` commits,
`Cancel`/Escape/the X discard, and clicking away with something typed swaps the form for a
three-way question (`Back`, `Discard changes`, `Save changes`) rather than stacking a second
dialog. A GM may change all of it on any token while a Player may change the trackers and
conditions on one they own),
fog the GM reveals and hides by dragging a rectangle (a plain click covers just the one cell under
it), starting fully revealed on a new scene rather than fully covered — a Player looking at a
scene nobody has painted fog on yet sees the map, not an unexplained black rectangle — plus
reveal-all and reset for the whole scene. **Fog is stored and sent as the set of *hidden* cells,
packed 32 to an integer** (`internal/store/fog.go` and `web/src/lib/fog.ts` are the two halves of
that format), which is why a new scene comes up revealed without anything having to materialise
it, and why a fully covered map costs 1,400 rows rather than 40,000. The GM's own cover opacity is
a slider on the fog family's strip, persisted per browser like the theme control rather than sent
anywhere,
drawings (freehand/line/rect/ellipse) with an eraser — a rect or an ellipse can be **shaded inside**
as well as outlined, from a paint-bucket toggle (`Fill shape`) that appears on the draw strip only for those two, and the
shading is translucent so the map still reads through it while the outline stays solid; every
drawing also carries its own **stroke width**, picked from three named sizes (Thin/Medium/Thick,
each shown as a bar of the weight it makes) behind one button on the same strip, which shows the
width it is on and opens the three on a click — offered to every drawing tool but the eraser. The
widths live in `web/src/lib/stroke-width.ts` and the popup in
`web/src/lib/components/stroke-width-picker.svelte`; a range input was ruled against in favour of
the strip's own discrete-button idiom, and the three were put behind a button to keep the strip
short. **The colour is a second button of the same shape and the same rule** — it wears the colour
it will draw in, drops out for the eraser beside the width, and opens **eight swatches in two
rows**: black/red/green/blue for light map art, white/bright
red/bright green/bright blue underneath for dark, in columns so each sits under the one it answers
to. Both rows are always shown and nothing reads the theme: the scheme says what the *page* is
wearing, and a dark battle map under a light UI is the case the second row exists for. The colours
live in `web/src/lib/stroke-colors.ts` and the popup in
`web/src/lib/components/stroke-color-picker.svelte`; the swatches sat on the strip itself until
the second row made it 66px tall, which is a band of map the strip covers in the corner the art
usually starts in. The border on every swatch is what keeps white off a light panel and black off
a dark one — and per-session undo/redo covering
drawing, erasing, token creation, token deletion, token edits and token moves (an undo passes over
a token someone else has changed since, rather than dragging it back out from under them),
pings, distance measuring, area-of-effect templates (circle/cone/line/cube, origin on a snap
mode and size in whole 5 ft steps),
**an initiative tracker** in the rail's second panel — the GM's alone to change, everyone's to
read: entries either stand for a token (taking its name and art, and its visibility with them) or
stand alone for a lair action or a hazard, sorted by the value rolled with a manual nudge for
ties, a current turn and a round counter that wrap together, and a two-click clear that leaves the
tokens on the map. It belongs to the room rather than the scene, so a GM switching to the battle
map mid-fight keeps the encounter,
chat with `/roll` and a two-stage delete (a GM may delete or purge any message, everyone else
only their own — the first delete leaves the room seeing "this message has been deleted", while
the author and whoever just deleted it still see the original text struck through; a second
delete on that same message purges it outright for everyone — that bin is invisible and
click-through until the message is hovered, focused or tapped, since a transparent button that
still takes clicks lets a first tap on a phone delete something never seen). **Every entry carries
the time it
landed**, with the full date on hover and the date above the first entry of each day (`Today`,
`Yesterday`, else the date), and the log also records **who came and went**: a `system`
message kind the hub writes itself, holding the event (`joined`/`left`) rather than a sentence, so
the wording stays a `longtable-copy` decision. Those lines are the room talking — no bold name, no
delete button — and they persist, so a refresh and a late arrival read the same history.
**Everyone at the table has a colour**, picked from sixteen presets on the same form as their name —
on the seat picker before joining, where the swatch beside each chair says which colours the room
is already wearing (taken ones are marked, never blocked: two people may match, and the room
doesn't argue). It belongs to the *seat*, so it survives a cleared browser and comes back when that
seat is taken on another device, and it shows up in two places that answer "who": the name in chat,
and the colour a ping pulses in. It can be changed later from the swatch beside `playing as` in the
rail — `participant.setColor` names no seat, so it can only be your own — and the change recolours
what you already said, since a name's colour is looked up per render rather than stamped on a
message. A seat from before colours has none and renders exactly as it did.
And a live list of who's connected
(distinct from the room's roster of everyone who has ever joined, which `state.sync` also
carries).

**Presence is the hub's, and leaving is on a timer.** A participant whose last connection closes
stays present for `departure_grace` (30 seconds by default); coming back inside that window
cancels it and broadcasts *nothing at all*, since the room was never told they left. Only when it
expires do `participant.disconnected` and the chat log's `left` line go out. That is what stopped
the badges flickering every time a phone locked its screen — the reconnect backoff starts at half a
second and doubles towards fifteen — and it is what makes a durable "left the room" line mean
something rather than recording every wobble on the wifi. `ConnectedParticipantIDs` counts anyone
mid-grace as connected, which is load-bearing: a resumption is silent, so a client that synced
during someone's window would never get the arrival that corrected it.

**The room page is built around the map**: the canvas fills the window, with no page padding, no
card and no header. The toolbar floats over its top-left as five tool *families* — hand, draw,
measure, fog, ping — with `New token` alongside and a contextual strip below carrying only the
active family's variants and settings (the eraser is inside draw; the templates inside measure).
Everything else lives in a fixed full-height rail down the right: the selected token at the top
(a plain shaded block holding its height when nothing is selected, so the rail doesn't jump),
session info under it (room name, who you are, the socket status),
then chat or the initiative tracker filling the rest, and three icons at the foot switching
between chat, initiative and a menu. That menu opens with **the room code**, shown in monospace
under a muted label and readable without going further; clicking it opens a dialog holding the code
and this browser's address as readonly fields, one click to select either. There is no copy button
anywhere, and that's a decision, not a gap — see
`planning/backlog/share-room-code-from-room.md`. Under it the menu holds Scenes, Assets, Manage
room and Leave room (making a scene is a mode of the Scenes dialog rather than a menu entry of its
own), and above Leave room a **System/Light/Dark** control — grouped with it because those two are
the only things in the menu that change this browser rather than the room. Below `lg` the rail becomes a bottom sheet with those icons pinned to the bottom edge, the
contextual strip docks into it rather than floating, the selected token becomes a bar above the
icons shown only when something is selected, and redo and reset view move from the toolbar into
the menu. A dropped socket reconnects on
its own with backoff, and says so on a banner across the top of the map while it's down — the one
thing a Room Member must not miss, and a status dot in a corner is missable. Art enters a room only through the
assets page at `/r/{slug}/assets`. That page is tabbed by kind, Tokens and Maps, and the tab
governs all of it: what the library grid shows, and what anything added from there is filed as —
chosen before the file dialog opens rather than asked for afterwards. An upload is named
(defaulting to the filename minus its extension), credited, and — for a map — aligned to the grid
before it's added; if its dimensions disagree with the tab it was staged under, the card says so
and offers to move it, but never moves it on its own. Name, credit and kind are all editable
afterwards, and an asset can be removed from a room's library (the shared file survives, and
anything already using it keeps it). Token art is shown whole in a square tile, maps in a wide
crop; the library there and the pickers in the room share one searchable component, the pickers
show only the kind they're asking for (no tabs — a scene picks a map, a token picks token art),
and the pickers only pick — their
link out to the assets page carries the open tab with it (`?kind=`), so art added after following
it is filed as the thing that was being looked for. Every
upload is decoded and re-encoded to WebP, with any grid offset padded into the pixels on the way
through, and joins the uploading room's library — content-addressed globally so identical uploads
share one file, but a room only ever sees what it added itself, under its own name and credit.
All of it syncs live; everything but pings and measurements persists.

**The app has a dark scheme**, and it follows the device unless told otherwise. `mode-watcher` in
`+layout.svelte` puts the `dark` class on `<html>` and keeps it in step with the OS live; a
System/Light/Dark control overrides that per browser, stored under `longtable:theme`. It is three
icon buttons — sun, moon, and a monitor for System — and appears twice: as a labelled row in the
room menu, and as a pill floating in the **bottom-right** corner of the home page. Bottom-right
rather than the usual top-right because the Host's banner is `fixed` across the top of every page,
and a control up there would either collide with it or have to read `hostNotice.height` and move. There is deliberately **no options page** — one control
doesn't earn a route, and a full-bleed room would have needed a menu entry to reach it anyway
(`planning/backlog/options-page.md` records why it was dropped). Two things are worth knowing
before touching any of this. The inline boot script in `web/src/app.html` is what stops the white
flash, and it can't be moved into a component: `ssr = false` means nothing a component renders
reaches the served HTML, so the scheme has to be applied before the app exists. And on the canvas,
`mode.current` is read in exactly **one** effect, which assigns a plain `stageScheme` the render
functions read — a reactive read inside a render function gives every effect that calls one a
dependency on the theme, which is the same trap `resetView` carries a comment about. Only two
things Konva paints follow the scheme: the grid, and the slab shown where a scene has no map.
Strokes, pings and the rest are map content and stay put.

On startup the server prints the LAN addresses players can use, one line per interface with the
interface's name — a Host binding `-addr` to one interface is answered with that address alone,
since enumerating the rest would be a lie about where the server is.

**A Host configures the server by editing one file.** `longtable.toml`, in the working directory
beside the database and assets it names, written with defaults and a comment per setting the first
time the server runs and never rewritten after that — so a Host's own notes survive. `-config`
points at another one and is the only flag any subcommand takes; there are no environment
variables and no setting flags. A key the server doesn't recognise stops it starting and prints
the line the typo is on, because the failure a config file invents is an edit that silently does
nothing. A key that's simply absent takes its default, which is what lets a later version add one
without breaking every file already out there.

**The Host's two settings that reach a room** are `banner` and `departure_grace`. The banner puts
a message across the top of every page for everyone on the server, dismissable per browser and
keyed by its own text, so changing it brings it back for people who dismissed the last one. A Host
runs the server and needn't be at any table on it (`planning/roles.md`), which is why it's a
setting rather than a screen. `departure_grace` is the same shape of decision: how long a dropped
connection has to come back before the room is told, where the right answer is the hall's wifi
rather than ours. Neither is re-read while the server is up — changing either means a restart, and
`planning/backlog/host-config-file.md` records why live reload was left out.

Known gaps, which is also roughly the queue: nothing rolls initiative for you — the tracker takes
the number and `/roll 1d20+2` in chat is where it comes from; `Manage room` holds seats, the
movement lock, the room password and deleting the room, and is still waiting on room privacy and
a switch to turn Player token creation off. Nothing caps how many tokens one Player may have standing. Fog has no automatic vision from tokens, no prebuilt releases, no way for a Host
to remove a moderated asset server-wide or cap upload sizes per room (a room removing something
from its own library is a different, smaller thing, and does exist). The
theme control isn't on the pre-join screen or the assets page — both are passed through rather
than sat in, and both are one step from somewhere that has it.

Two bugs used to bite specifically when the app was used the way it's meant to be — a GM hosting
and everyone else on `http://192.168.x.x:8080` from their own device — while never showing up on
localhost or in the test suites. **Both are fixed**, and both are worth knowing about, because the
next thing to break on a tablet will break the same way: silently, for everyone except whoever is
developing it.

The map can now be pinched to zoom on a touch device (`handlePinchMove` in `game-canvas.svelte`,
arithmetic in `web/src/lib/pinch.ts`). Until then the only thing that scaled the stage was a mouse
wheel, so a Player on an iPad saw about nine squares of a battle map and had no way to pull back.

**The map also pans on a right- or middle-button drag, in every tool** (`handlePanStart` and its
neighbours in `game-canvas.svelte`, arithmetic in `web/src/lib/pan.ts`). Panning was a left-drag
and so existed only in the Hand tool, since a tool and a pan both open on a left press; moving the
map mid-measurement meant a trip back to the toolbar and another one back. The right button was
free by the decision in `planning/backlog/no-draw-on-right-click.md`, and this is the "other
purpose" that story left room for — so the browser's context menu is now suppressed across the
whole stage, and `Konva.dragButtons` is narrowed to `[0]` so Konva's own drag stops answering to
the middle button.

Ids used to be minted with `crypto.randomUUID`, which browsers expose only in a
secure context, so drawing and pings threw for every client on a LAN address. They now go through
`randomId()` in `web/src/lib/random-id.ts`, which falls back to `crypto.getRandomValues` — read its
doc comment before minting an id anywhere else, and **never call `crypto.randomUUID` directly**.
Anything else gated on a secure context has the same trap waiting. **`navigator.clipboard` is the
next one and has already been ruled against**: a copy-the-room-code button was built with an
`execCommand` fallback, passed unit and e2e tests, and was deleted before it shipped, because
nothing in this repo can test the case it exists for — Playwright drives localhost, which is a
secure context. The room shows the code and points at the address bar instead. Read
`planning/backlog/share-room-code-from-room.md` before reaching for the clipboard here.

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
| E2E | `cd web && npx playwright test` — builds the frontend, builds the Go binary that embeds it, and tests against that one binary (not the dev server; see `testing.md`) |

Scope the Go commands to `./internal/... ./cmd/...`, not `./...`: the repo root also contains
`web/node_modules`, which the go tool would walk looking for packages. CI (`.github/workflows/ci.yml`)
runs all of the above, with `-race` on the Go tests — `-race` needs cgo, so it can't run on this
Windows box, but it does run on Linux CI.

## Before touching a shared port

The e2e harness binds **:8080** — the Go binary, serving the SPA it embeds; it no longer starts
vite, so :5173 is only in play if someone is running `npm run dev` by hand. Other Claude sessions
work in this same checkout, so a process already on either port may well be in use — ask before
stopping anything. `npx playwright test` writes to `web/.e2e-data/longtable.db`, which it wipes at
the start of every run — so starting a run pulls the data out from under any other session
reading it, and what's left afterwards is the last run's alone.

## House style

The thing that most distinguishes this codebase: **comments explain why, not what.** Nearly every
non-obvious line has a note on the constraint or failure that produced it — the `track()` helper
existing because `$effect` can't see reads after an `await`, `isCanonicalUUID` rejecting the
braced spelling so an echoed id stays byte-identical, the LIFO defer ordering in `ServeHTTP`.
When you change something with a reason behind it, leave the reason. When you find such a
comment, treat it as load-bearing: it's usually recording a bug someone already hit.

Others worth matching:

- Prose in comments and planning docs uses em dashes and reads like sentences, not telegraphese.
  **This rule stops at the code.** Anything a user reads — a button, an error, a line of the
  README — is written the other way, and `longtable-copy` is the authority on how. Applying this
  bullet to a button is a specific, repeated mistake, not a stylistic difference of opinion.
- Go errors: `slog.Error` with the internal detail, then a short human message to the client.
  Never leak the internal error over the socket.
- **No `CHECK` constraint on a column whose set of values could ever grow**, which so far means no
  `CHECK` anywhere. SQLite can't alter one, so widening it is a table rebuild — and the failure
  lands on databases created *before* the change, which is every real one and none of the test
  ones. The five this schema used to carry (role, token visibility, drawing kind, asset kind,
  message kind) were all duplicating a check Go already does on the way in. Keep the enforcement
  there, where an error can also say something useful to the client, and name the authority in a
  comment on the column. `TestSchema_HasNoCheckConstraints` fails if one comes back.
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
- `.claude/skills/longtable-copy/` — how to write anything a *user* reads: on-screen strings, and
  `README.md` and `docs/`. Read it before adding or renaming a string, however small. The house
  style below is for comments and planning prose and is actively wrong on a button — that skill
  exists because the two kept getting mixed up.

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
| `.claude/skills/longtable-copy/SKILL.md` | how user-facing text is written — on-screen strings, `README.md`, `docs/`, and the worked before-and-afters the rules come from |
| `planning/` | why a thing exists, what shipped, what's next |

`planning/backlog/README.md` and `planning/user-stories/README.md` restate those same rules for
someone browsing the folder without the skill loaded. Three files saying one thing is exactly the
duplication this table exists to catch, so treat the skill as the authority and the two READMEs as
its summary — when the convention changes, all three move together or the odd one out starts
lying.

Update them **in the same commit as the change**, not in a later sweep — a doc corrected a week
after the fact has already misled someone. The triggers, all cheap:

- Added a package, or moved something in the table above → the layout table here.
- Wrote or reworded anything a user reads — a string, a README line, a paragraph of `docs/` →
  `longtable-copy` first. If the wording gets rewritten afterwards, add the before-and-after to
  that skill's tables in the same commit; the examples are what make it accurate.
- Added or changed a WS command or event, or who's allowed to send one → the command table in
  `ws-protocol.md`.
- Added a Konva layer or a tool → the layer table in `canvas.md`, **and** the layer-order
  comments in `web/e2e/*.spec.ts`, which index layers by number.
- Added a test helper or changed how a suite runs → `testing.md`, and the README if the command
  a human types changed.
- Added a Host-configurable setting → `internal/config`'s struct and template **and** the table in
  `docs/hosting.md`. The generated file and that table are the only two places a Host learns a
  setting exists, and the story behind them (`host-config-documentation`) asks for both to stay
  current.
- Shipped a feature or closed a gap → "Where things stand" above, plus flipping the backlog item's
  `status:`, its "What shipped" note, and flipping the linked user story's `status:` to `done`
  once every acceptance criterion actually holds (see `longtable-backlog`).
- Changed how `planning/` itself works — status values, file format, where a thing lives → the
  `longtable-backlog` skill **and** both `planning/` READMEs, in one commit.
- Found one of these docs contradicting the code → fix it then, even mid-task. It's a two-line
  edit now and an hour of someone's confusion later. If the contradiction is in a `planning/done/`
  note, correct it in place rather than deleting the old text: the reasoning that turned out to be
  wrong is usually why the code changed.
