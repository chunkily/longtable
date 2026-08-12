# The WebSocket protocol

Everything here is `internal/ws/hub.go` unless noted. Read alongside `internal/ws/state.go`
(`state.sync` and `scene.activated` payload building).

## Connecting

`GET /ws?room=<slug>&token=<sessionToken>`. The slug resolves the room, the token resolves the
participant — there is no way to reach a room's live state without a valid session, mirroring the
REST endpoints. Sessions are per-room and live in `localStorage` (`web/src/lib/session.ts`); there
are no accounts. On connect the server immediately sends `state.sync` and then loops on reads.

The token resolves to a participant through the `session` table rather than off the participant
row: a participant is a **seat**, and many devices can hold one over time (ADR-0008). Nothing in
the protocol changed for it — `c.participant` is still the identity every handler uses, and
`GetParticipantByToken` is still the single place a credential becomes one. What did change is
that two *different* tokens can now resolve to the same participant, which is what makes a phone
and a laptop one person; see the presence note below, which was already written to survive it.

Envelope both ways:

```json
{ "type": "token.move", "payload": { "tokenId": "…", "x": 3, "y": 4 } }
```

## Commands (client → server)

| Command | Who | Persists | Broadcast |
| --- | --- | --- | --- |
| `chat.send` | anyone | yes | `chat.posted` |
| `chat.delete` | author, or any GM | yes | `chat.deleted` (first call) or `chat.purged` (second) |
| `token.create` | anyone; a non-GM's is theirs and visible | yes | one `token.created` per token (GM-only if hidden) |
| `token.move` | anyone, or owners only when the room is locked | yes | `token.moved` |
| `token.update` | GM, or the token's owner for trackers/conditions | yes | `token.updated` (+ `token.deleted` to Players on hiding) |
| `token.delete` | GM, or the token's owner | yes | `token.deleted` (GM-only if hidden) |
| `fog.reveal` | GM only | yes | `fog.revealed` |
| `fog.hide` | GM only | yes | `fog.hidden` |
| `fog.revealAll` | GM only | yes | `fog.revealed` |
| `fog.reset` | GM only | yes | `fog.hidden` |
| `scene.create` | GM only | yes | `scene.created` (+ `scene.activated` if it's the room's first) |
| `scene.setActive` | GM only | yes | `scene.activated` |
| `scene.delete` | GM only | yes | `scene.deleted` |
| `scene.setMap` | GM only | yes | `scene.updated` |
| `room.setOwnerOnlyMovement` | GM only | yes | `room.updated` |
| `initiative.add` | GM only | yes | `initiative.updated` |
| `initiative.update` | GM only | yes | `initiative.updated` |
| `initiative.remove` | GM only | yes | `initiative.updated` |
| `initiative.reorder` | GM only | yes | `initiative.updated` |
| `initiative.advance` | GM only | yes | `initiative.updated` |
| `initiative.clear` | GM only | yes | `initiative.updated` |
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

`chat.delete` folds two delete stages into one command, the way `token.update` folds several
permission levels into one: which one fires depends on the message's current state, not on
anything the client sends. The first call stamps `deleted_at` and `deleted_by_participant_id` but
leaves the row's content alone, broadcasting `chat.deleted` **per client** — same technique as a
hidden token's broadcast — so the author and whoever just deleted it keep seeing the original
body (and roll fields, for a `/roll`) struck through client-side, while everyone else gets a
payload with those fields blanked and renders the generic "this message has been deleted"
placeholder instead. A second call on that same message purges the row outright and broadcasts
`chat.purged` — one payload for everyone this time, since there's no content left to redact.

Authorization for *both* stages is the same author-or-GM check against `Message.ParticipantID` —
that's who's allowed to *act*. Who's still allowed to *see* the content after the first stage is a
different question, decided by `messagePayload`'s `messageVisibleTo`: the author or
`Message.DeletedByParticipantID`, which is not always the same person — a GM moderating someone
else's message is a deleter who isn't the author, and stays privileged without that making the
message privileged for GMs generally. The row surviving the first stage is what makes any of this
possible: purging immediately would leave the second delete with nothing to check authorship
against, forcing it to fall back to GM-only and quietly taking a Player's own second click away
from them, and would leave `messageVisibleTo` with no content to redact selectively in the first
place. A message with no recorded author (like an unattributed drawing) is nobody's to delete but
a GM's, at either stage — and once deleted, the GM who did it is the only one still privileged.

### Fog

Fog is the set of **hidden** cells, packed 32 to an integer. Its unit everywhere except the two
painting commands is the `FogChunk` — `{y, chunkX, mask}`, where bit *n* of `mask` is the cell at
`x = chunkX*32 + n`, set when hidden. A chunk that isn't stored (or that reaches mask 0, at which
point it's dropped) is 32 revealed cells, so **absent is the only spelling of revealed** on both
sides of the wire. `internal/store/fog.go` and `web/src/lib/fog.ts` are the two halves of that
format and have to agree bit for bit; both shift and mask rather than divide, so a cell at a
negative x floors into the chunk below zero instead of colliding with a positive one.

Storing what's hidden rather than what's revealed is what makes a new scene cost nothing — it
comes up fully revealed because it holds no rows, not because anything materialised them. Packing
is what keeps a deliberately fogged one cheap: a 200x200-cell map fully covered is 1,400 chunks
rather than 40,000 cells, in the table *and* in the payload every client receives.

`fog.reveal` and `fog.hide` take cells (`{sceneId, cells}`) because cells are what the rectangle
tool paints in; the server groups them into chunks. Both are idempotent, and both broadcast only
the chunks whose mask actually **changed**, at their new value — so a drag over ground already in
the target state broadcasts nothing at all. `maxPaintedCells` caps the incoming list, since a
corner-to-corner rectangle names every cell inside it.

Both whole-scene buttons reuse those two events rather than having their own, so clients need no
extra case:

- `fog.revealAll` deletes the scene's rows and broadcasts `fog.revealed` with every chunk it
  removed, zeroed. It needs no scene bounds and has no cap — it only has to describe chunks that
  actually hold fog, however large the map is.
- `fog.reset` covers everything, and is therefore the one that enumerates the scene's bounds
  (`sceneFogChunks`, capped at `maxFogChunks`) and is refused for a scene with no width/height or
  no grid. Its `fog.hidden` delta also carries any chunk *outside* those bounds zeroed — fog
  painted left of or above the origin — or a client would keep drawing fog the server just
  deleted.

The cap and the bounds check used to sit on `fog.revealAll` and the free `DELETE` on `fog.reset`.
They swapped places when fog started storing what's hidden, which is worth knowing if an old
comment says otherwise.

Only the room's *first* scene auto-activates. A later `scene.create` is prep work, so it stays
off screen until the GM switches to it — before the scene picker existed it activated
unconditionally, because activation was the only way to ever reach a scene again.
`scene.delete` refuses the active scene: `room.active_scene_id` has no foreign key to clean it
up, so deleting it would leave every client pointed at a scene the server can't load.

`token.create` is **open to Players**, with the same per-field shape `token.update` uses: a
non-GM's owner is forced to the creator and the visibility to `visible`, and both are *ignored
rather than rejected* when the payload says otherwise. Owner comes from `c.participant.ID`, so
`requireOwnerInRoom` is skipped for that path — the sender is in the room by construction.

It carries a `count` (absent means one, capped at `maxTokensPerCreate` = 20 **server-side**, since
a stepper is a convenience and not a permission) and an optional `tokenIds` list, which must be
exactly `count` canonical UUIDs. A count above one numbers the tokens `Name 1` … `Name N` and
spreads them over free squares by Chebyshev ring (`internal/ws/spawn.go`); a count of one keeps
the name exactly as typed and lands on the requested square **even if something is standing
there**, which is what makes it safe for the undo of a deletion. Each token gets its own
`token.created`, so hidden-token filtering stays per recipient and every client folds them in one
at a time.

`tokenIds` replaced the old single `tokenId`. Two callers, one meaning — "the ids these tokens
must appear under": undoing a deletion, so the token comes back as the token the room still knows;
and a fresh batch, so the creating client can put one undo entry per token on its stack without
having to guess which of the arriving `token.created` events are its own (they carry no sender).

`token.delete` is a GM, **or the token's owner** — not the "your own work" rule `draw.delete`
uses, since a token has no author, but the same ownership that already governs its trackers. It
was GM-only precisely because creation was, and leaving it there once Players could conjure eight
monkeys would have made the clearing-up the GM's. A hidden token is refused to a non-GM in the
words of one that doesn't exist, *including to its owner*, exactly as in `token.update`. Its
broadcast is withheld from Players when the token was hidden, exactly as its creation was; an id
they were never told about turning up in a deletion is itself the leak.

`room.setOwnerOnlyMovement` is the room's first real setting, and `room.updated` carries the
**whole room** (`roomPayload`, shared with `state.sync`) rather than the field that changed — so
the next setting to land needs no new event and a reloading client sees one shape. It is built
field by field, like `participantPayload`, so the password hash can't ride along to every client.

`token.move` is open to everyone by default. When the room's `ownerOnlyMovement` is set, a non-GM
may move only a token they own; the GM is outside it, or turning the lock on would take the
monsters away from the only person who moves them. `mayMoveToken` loads nothing at all for a GM
and only the room for an unlocked table, so the common case pays one cheap read per drag. A hidden
token is refused as a missing one, the same sentence `token.update` and `token.delete` use — but
only when the lock is on, since an open room has never checked visibility on a move and turning
that into a refusal here would be a second, unasked-for feature.

`token.move` is also how a move is *undone* — the client sends the token back to the square it
came from, so any permission check added to `handleTokenMove` governs the undo for free (there is
a test asserting exactly that for the lock). The
broadcast carries no sender, which is why the client's history has to decide "was that my move?"
from the position rather than from the event: see `sendMoveToken` in `room.svelte.ts`.

`token.update` carries *every* editable field each time (name, image, size, owner, visibility,
trackers, conditions) rather than only the changed ones — a `*string` can't tell "left alone" from
"cleared", and clearing a token's art, or taking it back off a Player, is a real edit. The
corollary is that an update omitting a field **clears** it, so a client must send them all. It
deliberately doesn't carry position: that's `token.move`'s, so an edit dialog opened before a drag
can't undo the drag when it's submitted after one. The handler still loads the token and edits it
in place, which is what keeps a field this command doesn't mention from being nulled by a form
that predates it.

**It is the one command with a per-field role check** rather than a single gate at the top. A GM
may change anything; a Player who *owns* the token may change its `trackers` and `conditions` and
nothing else — tracking your own damage shouldn't need asking for, while who can see the token,
who owns it and what it looks like stay the GM's scene. Three things fall out of that and are
worth not rediscovering:

- The fields a Player may not touch are **ignored, not rejected**. The loaded token keeps them,
  exactly as it keeps a field the command doesn't carry. Rejecting instead would mean diffing
  every echoed field against what's stored and calling any difference an attack, which turns a
  stale form into an error and protects nothing extra.
- `name` is required only of a GM, since it's one of the fields a Player has no business sending.
- A **hidden** token is refused to a non-GM in the words of one that doesn't exist, *including to
  its own owner* — a GM can prep an ambush with a Player's character, and an error separating "not
  yours" from "no such token" would be how they found out. A visible token they don't own gets a
  plainer "you can only edit a token you own", which leaks nothing they can't already see.

`trackers` is a fixed **three** slots (`store.TrackerSlots`), each `{label, value}`. The server
pads or truncates to three, so a client never has to; more than three is an error rather than a
silent truncation, since a client sending four disagrees about how many there are and dropping the
last would lose whatever was just typed. `value` is `null` for an empty slot and `0` for a
creature on nought hit points — **the two must never collapse**, which is why it's a pointer on
the wire, in the store and in the TypeScript type. Values are integers: every number on a D&D
sheet is one. `conditions` is free-form text, trimmed, blanks dropped, deduplicated
case-insensitively keeping the first spelling, and bounded by `maxTrackerLabel`,
`maxConditionText` and `maxTokenConditions` in `hub.go` — the client mirrors those as `maxlength`
so a cap never turns into an error toast after someone has finished typing.

`token.create` carries both as well, though a token is normally created blank. Undoing a deletion
rebuilds the row from that payload alone, and a token that came back on full health would be a
worse bug than the misclick being undone.

The inline tracker boxes in the details panel send this same command, through
`RoomClient.setTokenTrackers`, which fills every other field in from the token as that client
holds it — there is no narrower "just the trackers" command, because one would read as a GM
clearing the name.

**`updateToken` drops a change that changes nothing**, comparing against the token as this client
holds it (`sameTokenFields`). That is not only tidiness: an editor left open while somebody else
worked on the same token would otherwise stamp its stale copy of every field over their edit the
moment it was submitted, since this command carries all of them.

It is also **undoable**, which took an entry holding both sides — the reverse of an edit is
another edit, and `token.update` carries no history, so "what it was" is only knowable at the
moment it stops being true. The undo is skipped if the token no longer matches what this session
last set, the same rule `token.move`'s undo follows and for the same reason: putting our version
back over someone else's change would be undoing their work rather than ours.

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

Ownership now governs four things: `token.update`'s trackers and conditions, `token.delete`, who a
Player's new token belongs to, and — when the room is locked — `token.move`.

## The initiative tracker

Six commands, all GM-only, all answering with **the whole tracker** rather than a delta —
`internal/ws/initiative.go`, kept out of `hub.go` only because that file is long enough that the
token handlers are already hard to find. Three reasons for sending it whole, in order of how
expensive they'd be to work around: entries are withheld per recipient, a turn advance changes the
round as well as the pointer, and a removal can move whose turn it is. Most "small" changes are
several fields, and a tracker is a couple of dozen rows that change once a turn.

`initiative.updated` and `state.sync`'s `initiative` key carry `{entries, round, currentEntryId}`.
An entry is `{id, tokenId, name, initiative, hidden, imageAssetId}`.

**The tracker belongs to the room, not the scene.** A GM flipping to the battle map mid-fight must
not lose the encounter, so entries hang off `room_id` and the turn/round live in two columns on
`room` — one row's worth of state, where a second table would be a join for two scalars.

**An entry is either a token or a name.** A linked one resolves its name and art from the token on
every send rather than copying them at creation, so renaming a token renames its entry; a
freestanding one is for lair actions and hazards. That live resolution is why **any change to a
token re-broadcasts the tracker** if an entry stands for it (`broadcastInitiativeIfLinked`), and
why `handleTokenDelete` has to ask *before* deleting: `initiative_entry.token_id` is
`ON DELETE CASCADE`, so afterwards there is no entry left to notice.

**Two ways to be invisible, one answer.** A freestanding entry has its own `hidden` flag; a linked
entry's visibility is its token's, read from the token so the two can never disagree. Players are
never *sent* either kind, so counting rows tells them nothing. `currentEntryId` is still sent when
it names a withheld entry — the Player sees a tracker with nothing highlighted, which says "it's
somebody's turn and not yours" without saying whose.

**The round changes only at the wrap, in both directions** (`advanceTurn`), which is what makes
next-then-previous land exactly where it started across a round boundary. The first press of Next
with nobody up starts at the top of the order without counting a round nobody has played, and
round 1 is the floor.

`initiative.reorder` moves an entry one place **only among the entries it is tied with**. That is
the feature, not a limitation: the order *is* the values, and letting an entry jump a higher roll
would make the list disagree with the numbers printed beside it. It renumbers `sort_order` across
the whole list rather than swapping two values — every new entry starts at 0, so a swap would
trade one zero for another and change nothing.

Notable gaps as of the last pass: there is no GM switch to turn Player token creation off, and no
cap on how many tokens one Player may have standing at once — twenty at a time, as many times as
they like. Nothing rolls initiative for you either: `initiative.add` takes the number, and `/roll
1d20+2` in chat is where it comes from. See `planning/backlog/`.

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
  was already here. Since seats this covers two *devices* as well, for free and for the same
  reason: the dedupe keys on the participant, and a phone and a laptop signed into one seat are
  two sessions pointing at one participant. `ConnectedParticipantIDs` is exported for the
  pre-join seat list, which needs the same live answer to say whether a chair is taken.
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

`state.sync`, `chat.posted`, `chat.deleted`, `chat.purged`, `token.created`, `token.moved`,
`token.updated`, `token.deleted`, `participant.connected`, `participant.disconnected`,
`fog.revealed`, `fog.hidden`, `scene.activated`, `scene.created`, `scene.updated`,
`scene.deleted`, `room.updated`, `initiative.updated`, `drawing.created`, `drawing.deleted`,
`ping`, `measure.updated`, `measure.ended`, `error`.

`state.sync` and `scene.activated` both carry the same full picture — `{scene, tokens, fogChunks,
drawings}` built by `sceneStatePayload` — so a client can render immediately without another
round trip. `state.sync` adds `room`, `you`, the last 50 chat messages (newest first; the client
reverses them), and `scenes`: every scene in the room, for the picker. `messagePayloads` builds
that list against the connecting client's own participant id, so a message soft-deleted before it
connects arrives already redacted (or not) exactly as it would have live — the author or deleter
sees the original content, everyone else `deleted: true` with the fields blanked. A purged one is
gone from the table and simply isn't among the 50.

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

`token.create` takes an optional `tokenIds` list through that same check, for a different reason:
nothing is rendered ahead of the server there, but undoing a deletion has to restore the token
under the id the rest of the room still knows it by, and a batch's creator has to know its own
tokens' ids to build its undo entries.

On the client, **mint ids with `randomId()` from `$lib/random-id`, never `crypto.randomUUID`
directly.** `randomUUID` is defined only in a secure context, and Longtable's whole deployment
story is players on `http://192.168.x.x:8080`, which isn't one — so calling it threw for everyone
but the GM, and neither the e2e suite nor a developer's browser could see it. The fallback there is
a real v4 UUID built from `crypto.getRandomValues`, which isn't gated the same way, and it produces
the canonical spelling on purpose: anything else would pass on localhost and be refused by
`isCanonicalUUID` on the LAN.
