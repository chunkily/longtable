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
| `scene.create` | GM only | yes | `scene.activated` |
| `scene.setActive` | GM only | yes | `scene.activated` |
| `draw.create` | anyone | yes | `drawing.created` |
| `draw.delete` | author, or any GM | yes | `drawing.deleted` |
| `ping` | anyone | no | `ping` |
| `measure.update` | anyone | no | `measure.updated` |
| `measure.end` | anyone | no | `measure.ended` |

Unknown command types get an `error` back naming the type. `chat.send` text starting with `/` is
routed to `handleSlashCommand` (only `/roll` and `/r` today; unknown commands error back to the
sender and never enter the chat log).

Notable gaps as of the last pass: there is no way to hide a revealed fog cell, delete a scene,
replace a scene's map, or move a token with an ownership lock. See
`planning/backlog/in-progress/`.

## Events (server → client)

`state.sync`, `chat.posted`, `token.created`, `token.moved`, `fog.revealed`, `scene.activated`,
`drawing.created`, `drawing.deleted`, `ping`, `measure.updated`, `measure.ended`, `error`.

`state.sync` and `scene.activated` both carry the same full picture — `{scene, tokens, fogCells,
drawings}` built by `sceneStatePayload` — so a client can render immediately without another
round trip. `state.sync` adds `room`, `you`, and the last 50 chat messages (newest first; the
client reverses them).

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
