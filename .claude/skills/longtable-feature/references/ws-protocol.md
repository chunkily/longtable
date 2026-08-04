# The WebSocket protocol

Everything here is `internal/ws/hub.go` unless noted. Read alongside `internal/ws/state.go`
(`state.sync` and `scene.activated` payload building).

## Connecting

`GET /ws?room=<slug>&token=<sessionToken>`. The slug resolves the room, the token resolves the
participant — there is no way to reach a room's live state without a valid session, mirroring the
REST endpoints. Sessions are per-room and live in `localStorage` (`web/src/lib/session.ts`); there
are no accounts. On connect the server immediately sends `state.sync` and then loops on reads.

Envelope both ways:

```json
{ "type": "token.move", "payload": { "tokenId": "…", "x": 3, "y": 4 } }
```

## Commands (client → server)

| Command | Who | Persists | Broadcast |
| --- | --- | --- | --- |
| `chat.send` | anyone | yes | `chat.posted` |
| `token.create` | GM only | yes | `token.created` (GM-only if hidden) |
| `token.move` | anyone | yes | `token.moved` |
| `token.update` | GM only | yes | `token.updated` (+ `token.deleted` to Players on hiding) |
| `token.delete` | GM only | yes | `token.deleted` (GM-only if hidden) |
| `fog.reveal` | GM only | yes | `fog.revealed` |
| `fog.hide` | GM only | yes | `fog.hidden` |
| `fog.revealAll` | GM only | yes | `fog.revealed` |
| `fog.reset` | GM only | yes | `fog.reset` |
| `scene.create` | GM only | yes | `scene.created` (+ `scene.activated` if it's the room's first) |
| `scene.setActive` | GM only | yes | `scene.activated` |
| `scene.delete` | GM only | yes | `scene.deleted` |
| `scene.setMap` | GM only | yes | `scene.updated` |
| `draw.create` | anyone | yes | `drawing.created` |
| `draw.delete` | author, or any GM | yes | `drawing.deleted` |
| `ping` | anyone | no | `ping` |
| `measure.update` | anyone | no | `measure.updated` |
| `measure.end` | anyone | no | `measure.ended` |

`measure.update` carries a `kind` — `distance` (the default when absent, so an older client is
unaffected) or one of the four area templates `circle`, `cone`, `line`, `cube`. An unknown kind is
refused. Only `line` uses `widthFeet`, since a drag gives length and direction but never width;
feet rather than world units so it means the same on a scene with a different grid size. The
templates share the distance line's whole lifecycle — one per participant, replaced on each
update, cleaned up on disconnect — so nothing else about the relay changes.

Neither the distance nor a template's outline is computed server-side, and both snapping (of the
origin) and 5 ft quantising (of the far end) are applied on the client *before* sending. The
points that arrive are final: a recipient renders what it's given and never has to know which
snap convention produced them, or that the length was rounded at all.

Unknown command types get an `error` back naming the type. `chat.send` text starting with `/` is
routed to `handleSlashCommand` (only `/roll` and `/r` today; unknown commands error back to the
sender and never enter the chat log).

`fog.revealAll` deliberately broadcasts `fog.revealed` rather than an event of its own: it
enumerates the scene's cells server-side (`sceneFogCells`) and sends them as an ordinary reveal,
so clients need no extra case and the server stays the only thing that decides what cells a scene
has. It's refused for a scene with no width/height or no grid, and capped at
`maxRevealAllCells` — the count is quadratic in map size and every cell is both a row and an
entry in the payload every client receives.

Only the room's *first* scene auto-activates. A later `scene.create` is prep work, so it stays
off screen until the GM switches to it — before the scene picker existed it activated
unconditionally, because activation was the only way to ever reach a scene again.
`scene.delete` refuses the active scene: `room.active_scene_id` has no foreign key to clean it
up, so deleting it would leave every client pointed at a scene the server can't load.

`token.delete` is GM-only, matching who may create one — deliberately *not* the "your own work"
rule `draw.delete` uses, since a token has no author to fall back on: it's a piece of the GM's
scene that a Player may merely be allowed to move. Its broadcast is withheld from Players when
the token was hidden, exactly as its creation was; an id they were never told about turning up in
a deletion is itself the leak. Undoing a deletion is a `token.create` carrying the original id,
so the token returns as the same token to everyone still holding it.

`token.move` is also how a move is *undone* — the client sends the token back to the square it
came from, so any permission check added to `handleTokenMove` governs the undo for free. The
broadcast carries no sender, which is why the client's history has to decide "was that my move?"
from the position rather than from the event: see `sendMoveToken` in `room.svelte.ts`.

`token.update` carries *every* editable field each time (name, image, size, owner, visibility)
rather than only the changed ones — a `*string` can't tell "left alone" from "cleared", and
clearing a token's art, or taking it back off a Player, is a real edit. The corollary is that an
update omitting a field **clears** it, so a client must send them all. It deliberately doesn't
carry position: that's `token.move`'s, so an edit dialog opened before a drag can't undo the drag
when it's submitted after one. The handler still loads the token and edits it in place, which is
what will keep a field this command doesn't mention yet (HP, conditions) from being nulled by a
form that predates it.

`ownerParticipantId` on both `token.create` and `token.update` is checked with
`requireOwnerInRoom`, the participant twin of `requireAssetInRoom` — a participant ID is
unguessable but it isn't scoped, and a token owned by someone in another room is one whose owner
nobody present can be shown. Null (nobody owns it) is always allowed and is what most tokens are.
Both handlers also read a missing `width`/`height` as one square.

That check is **membership of the room, not presence**, even though the UI only offers people who
are connected. Being offline isn't being gone: ownership outlives a session, and a rule keyed on
the connection registry would refuse an assignment the moment someone's socket blipped. Expect
the client to send owners the protocol accepts but the picker wouldn't have offered — an edit to
a token whose owner has since left does exactly that.

Its broadcast is the only one that depends on what the token *used to be*, because crossing the
hidden line has to say something different in each direction:

| Was | Now | GM gets | Player gets |
| --- | --- | --- | --- |
| visible | visible | `token.updated` | `token.updated` |
| hidden | visible | `token.updated` | `token.updated` (the whole token — they never had it) |
| visible | hidden | `token.updated` | `token.deleted` |
| hidden | hidden | `token.updated` | nothing |

Which is why **`token.updated` is an upsert on the client**, not a replace: a revealed token
reaches someone who has never held it. And why hiding sends Players a *deletion* of a row that
still exists — from their side that is exactly what happened, since a hidden token has never been
something a Player is told about. A dedicated `token.hidden` event would be more precise and buy
the client nothing.

Notable gaps as of the last pass: there is no way to move a token with an ownership lock, and no
way to assign a token's owner (blocked on listing a room's participants). See
`planning/backlog/in-progress/`.

## Presence

Two different questions, kept apart on purpose:

- **The roster** — everyone who has *ever* joined, from the `participant` table. Arrives in
  `state.sync` as `participants`. This is what an owner picker offers, including the Player who
  joined last week and isn't online now.
- **Who's connected** — live, in `Hub.rooms` and nowhere else. Arrives as
  `connectedParticipantIds`, then stays current through `participant.connected` /
  `participant.disconnected`.

Folding them into one "online" flag per row would make the offline half unrepresentable, which is
exactly the half a GM prepping tokens needs.

Three things worth knowing before touching it:

- **A person is not a connection.** Two browser tabs are two clients and one participant, so
  `register`/`unregister` return whether this was the *first* or *last* connection that
  participant had open, and only those broadcast. Otherwise opening a tab announces someone who
  was already here.
- **`participant.connected` is the one broadcast that skips its own sender.** Every other one
  echoes back because the sender has something optimistic to reconcile; here the arriving client's
  `state.sync` already lists it among the connected, so an echo would be a second copy of
  something it acted on a moment ago.
- **It carries the whole participant**, so a first-time joiner — who is on nobody else's roster
  yet — can be upserted. `participant.disconnected` carries only an id: they stay on the roster,
  because leaving the table isn't leaving the room.

`participant.disconnected` is sent from a fresh context, like the measurement cleanup, since the
request context is already cancelled by the time a connection drops.

The roster query deliberately never loads `session_token`, and `participantPayload` is an
exhaustive struct-to-map rather than a marshalled struct — two lines of defence against a
credential reaching every client in the room.

## Reconnecting

`GET /api/rooms/{slug}/session` (bearer token) answers 200 / 401 / 404 and exists only for the
client's retry loop. A refused WebSocket upgrade reaches the browser as a bare `onclose` with no
status, so the socket alone can't separate "the server is restarting, keep trying" from "this
session is gone, send them back to the join form". The probe makes it exact. It deliberately
doesn't echo the token back.

`RoomClient` retries with capped exponential backoff and jitter, reusing the saved session token
(re-opening the socket, never re-joining — joining again would mint a second participant). It
gives up after eight attempts in favour of a manual button, and stops immediately when the probe
says the session is the problem. The reducer side needs nothing: `state.sync` replaces the whole
picture and `resetAfterSync` drops anything in flight, so a reconnect converges by construction.

## Events (server → client)

`state.sync`, `chat.posted`, `token.created`, `token.moved`, `token.updated`, `token.deleted`,
`participant.connected`, `participant.disconnected`, `fog.revealed`, `fog.hidden`,
`fog.reset`, `scene.activated`, `scene.created`, `scene.updated`, `scene.deleted`,
`drawing.created`, `drawing.deleted`, `ping`, `measure.updated`, `measure.ended`, `error`.

`state.sync` and `scene.activated` both carry the same full picture — `{scene, tokens, fogCells,
drawings}` built by `sceneStatePayload` — so a client can render immediately without another
round trip. `state.sync` adds `room`, `you`, the last 50 chat messages (newest first; the
client reverses them), and `scenes`: every scene in the room, for the picker.

`scene.created` and `scene.updated` carry **only** `{scene}` — deliberately not the full picture.
A map swap changes the backdrop and nothing else, and sending `scene.activated` for it would make
clients treat it as a scene change and throw away undo history and any gesture in flight, which
is the opposite of the point: replacing a map keeps the tokens, fog and drawings on it.

## Delivery

- `broadcast(ctx, roomID, type, payload)` — everyone in the room, same payload. The sender gets
  its own echo; that's relied on for matching optimistic renders, and is why tests read one
  envelope off the sender before reading the recipient's.
- `broadcastPerClient(ctx, roomID, type, build)` — payload computed per recipient. `build`
  returning `nil` skips that client entirely. This is how hidden tokens are withheld from
  players: they are never sent, not even filtered-out or nulled.
- `send(ctx, c, type, payload)` — one client, for `state.sync` and errors.
- Clients are snapshotted under the mutex and written to outside it, so a slow write can't block
  the room.

## Validation, in the order handlers do it

1. `decodePayload(raw, &req)` and check required fields → `"invalid <command> payload"`.
2. Role: `requireGM(ctx, c)` for GM-only commands.
3. Scope: `requireSceneInRoom(ctx, c, sceneID)`, or `sceneInRoom` when the caller wants to word
   the error itself. For tokens, `TokenRoomID`; for a drawing, resolve it first and check
   `sceneInRoom(c, drawing.SceneID)` — its own scene, never a payload-supplied one.
4. References: `requireAssetInRoom(ctx, c, assetID)` so a scene or token can't point at a
   dangling asset id — or at an asset that exists but belongs to another room's library, since
   asset rows are global and content-addressed (see `internal/store/asset.go`'s `room_asset`
   table). `nil` (no image) is always allowed. `requireOwnerInRoom(ctx, c, ownerID)` is the same
   check for a token's owner: the participant table is global too, so existing is never the
   question — membership of *this* room is. `nil` (unowned) is always allowed.
5. Per-object permission, e.g. `draw.delete` letting a GM erase anything but a player only what
   they authored. A drawing with no recorded author belongs to nobody and is GM-only to erase.
6. Apply through the store, `slog.Error` on failure, short message to the client.
7. Broadcast.

## Errors

`sendError` gives `{"message": "…"}`. `sendDrawingError` adds `{"drawingId": "…"}` so a client
that already rendered a stroke optimistically knows exactly which one to take back — the same
mechanism would be worth copying for any future optimistic object. Internal errors go to
`slog.Error`; the client only ever sees a short human message.

## Ids

Clients may mint a drawing's id so the stroke they've already drawn can be matched to its echo.
`isCanonicalUUID` accepts only the lowercase hyphenated form — `uuid.Parse` alone also takes
braced and URN spellings, which would echo back an id that doesn't match the one the client is
holding. Any future client-chosen id should go through the same check.

`token.create` takes an optional `tokenId` through that same check, for a different reason:
nothing is rendered ahead of the server there, but undoing a deletion has to restore the token
under the id the rest of the room still knows it by, not a fresh one.
