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

Neither the distance nor a template's outline is computed server-side, and snapping is applied on
the client *before* sending. The points that arrive are final: a recipient renders what it's
given and never has to know which snap convention produced them.

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

Notable gaps as of the last pass: there is no way to move a token with an ownership lock. See
`planning/backlog/in-progress/`.

## Events (server → client)

`state.sync`, `chat.posted`, `token.created`, `token.moved`, `fog.revealed`, `fog.hidden`,
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
   table). `nil` (no image) is always allowed.
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
