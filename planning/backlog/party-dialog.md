---
title: Define the party in a dialog, and add it from the top of it
created: 2026-10-19
status: done
tags: [tokens, scenes, playtest]
story: gm-add-party-to-scene
---

GM feedback on [gm-party-token-presets](gm-party-token-presets.md) the day it shipped: rather than
marking tokens and pressing a toolbar button, the party icon should open a dialog where the party is
configured, with a button at the top that drops a copy of it onto the current scene.

Two choices settled with the GM before building:

- **The party is defined in the dialog**, not a view onto marked tokens. Each member is a row with
  name, art, size and owner, edited there; `Add from this scene` starts one from a token on the map.
- **`In the party` comes out of both token dialogs.** The dialog is the one place the party is
  configured.

## What shipped

The toolbar's party icon (past the divider, beside fog, GM-only) opens `Party`: `Add to this scene`
at the top, the roster under it with an edit and a remove icon per member, and `Add from this
scene…` and `New member` at the foot. The member form is a mode of the same dialog. Adding closes
the dialog so the GM watches them land; anyone already on the scene is skipped.

Decisions worth not rediscovering:

- **A `party_member` table, and `party_key` repurposed** as the id of the member a token was copied
  from — the thing that lets `party.add` skip a member already present. Not a foreign key, so
  removing a member leaves its copies on their maps as ordinary tokens. Keys minted by the first
  version name no member and are inert, so nothing was migrated; a GM who used it re-adds the party
  once, from the tokens it left, with `Add from this scene`.
- **`token.updated_at` was dropped**, through a new `droppedColumns` list beside `addedColumns`. It
  existed only to find "the newest copy" of a marked character, which a defined member makes
  meaningless. Dropped rather than left, since a column nothing reads is a question for every
  future reader of the schema.
- **The roster is a GM's alone**: `party.updated` goes only to GMs and only a GM's `state.sync`
  carries it. A Player sees the copies, never the list.
- **Owner offers every Player seat** (`partyOwnerOptions`), unlike a token's picker, which offers
  only who is connected. A party is set up before the session, when "who is here" is nobody.
- **No visibility on a member, and copies are visible.** The first version copied a token's
  visibility; a member has none to copy, and a GM hides a copy after it's down if they need to.
- **Removing a member arms and then fires**, like deleting a scene, saying `Really remove?` in words
  once armed.
- **`Add from this scene` is a native select**, acting on change and resetting — a menu would be a
  popover, and a popover inside a dialog doesn't work here.

**Attendance** came the same day, as a checkbox at the head of each row: only ticked members are
put down. It is a column on the member rather than state in the dialog, because a night's
attendance holds for every scene that night — a tick that cleared on every open would be re-ticked
per scene. New members start ticked, an untick dims the art and name (not the row — that made its buttons look disabled), and nobody ticked gets its own
message rather than "already here". The checkbox sends `party.memberUpdate` with everything else
unchanged; it isn't on the member form.

Building it found a bug older than the party: **the same error twice only toasted once.** The page
toasted when `RoomClient.error` changed, and assigning a string it already held is no change — so
pressing `Add to this scene` again on a scene the party was already on said nothing. `errorCount`
is bumped on every error and the toast is keyed on it; `party.spec.ts` presses twice and expects
two toasts, and fails without the fix.

Unchanged from the first version and still load-bearing: `party.added` for undo, `token.create`'s
GM-only `partyKey` for undoing a deletion, `spawnFootprints` for mixed sizes, and the client-side
scene filter fixed alongside it.
