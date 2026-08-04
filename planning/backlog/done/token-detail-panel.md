---
title: Token detail/inspector panel
created: 2026-07-29
tags: [tokens, ui]
---

Add a UI surface for viewing and editing a token's properties after creation. Today there's no
way to select a token and open its details — the only existing UI is the creation dialog
(`create-token-dialog.svelte`), and clicking a token on the canvas only supports drag-to-move.

This panel is a prerequisite for both assigning/reassigning a token's owner post-creation and for
editing HP/conditions.

Shelved: showing values on hover, and opening this panel directly from hovering or otherwise
interacting with the token. Instead the panel opens from an explicit button in the token details
section above chat (see [token-selection-highlight](token-selection-highlight.md)) — it
only ever opens on a deliberate click, once a token is already selected there, never from a
hover.

[token-selection-highlight](token-selection-highlight.md) has **shipped**, so both
prerequisites already exist: clicking a token on the canvas selects it, and the details section
above chat is there with room on its right for this panel's "Edit" button. Read that item's
"What shipped" before starting — the short version is that the selection lives in
`selectedTokenId`, a `$bindable` prop on `game-canvas.svelte` owned by the room page, and this
panel should read that same prop rather than grow its own.
[delete-token](delete-token.md) has since shipped too and put the first button in that
strip, so this panel's "Edit" goes beside it — and `token.delete` is the worked example of
adding a token command end to end, including the hidden-token broadcast filter this panel will
need if it can change a token's visibility.

## What shipped

A GM selects a token and presses **Edit** beside Delete in the details strip; a dialog edits its
**name, art, size and visibility**, and the change reaches the room and persists. Players get no
button — they can still select a token and read its details.

**What it deliberately does not do**, and why, because the item's three linked stories are all
still open:

- **Owner** is untouched: there is still no way to list who a room's participants *are*. That
  stays blocked on [list-room-participants](list-room-participants.md), which is now the
  only thing standing between this panel and `gm-assign-token-owner`.
- **HP and conditions** aren't here either. They need columns that don't exist, and the modelling
  (three labelled tracker slots plus condition tags) is
  [token-hp-condition-tracker](../in-progress/token-hp-condition-tracker.md)'s to do. What this
  item leaves it is the surface to put them on and `token.update` to carry them.

Decisions worth not rediscovering:

- **`token.update` sends every editable field every time, not a patch.** A `*string` on the wire
  can't distinguish "left alone" from "cleared", and clearing a token's art is a real edit. The
  handler then loads the token and edits it *in place*, so a field the command doesn't mention —
  owner today, HP tomorrow — survives a form that predates it. There's a test pinning exactly
  that, because it is the kind of thing a later field would silently break.
- **Position is deliberately not editable here.** It belongs to `token.move`. Folding it in would
  let a dialog opened before a drag undo the drag when it was submitted after one — also tested.
- **Crossing the hidden line is the one broadcast that depends on the previous state**, and it
  isn't symmetric. Hiding a token sends Players a `token.deleted` for a row that still exists,
  because from their side that is what happened; revealing one sends them the whole token, which
  is why **`token.updated` is an upsert on the client rather than a replace** — it arrives at
  someone who has never held it. The full matrix is in `references/ws-protocol.md`. A dedicated
  `token.hidden` event would be more precise and buy the client nothing.
- **The form loads from the token on open and does not track it afterwards.** Someone else
  renaming or moving the token mid-edit would otherwise overwrite what's been typed; on submit
  the form is the intent, so it wins.
- **No confirmation and no optimism.** Nothing renders ahead of the server, the same as
  `token.move` — a dialog has no preview shape that would blink.

The size control came out as its own component, `token-size-picker.svelte`, built to
`gm-set-token-size`'s categories so
[token-size-and-owner-pickers](token-size-and-owner-pickers.md) can drop it into
the creation dialog rather than build a second one. One change from that story worth flagging: it
lists six size names, but Tiny, Small and Medium are all 1×1, and three options that mean the
same thing can't be told apart when reading a token back — editing a "Tiny" token would silently
show it as "Medium". They are one option carrying all three names, which says the same thing and
round-trips.

## Related user stories

- [gm-assign-token-owner](../../user-stories/gm-assign-token-owner.md)
- [gm-set-token-hp-condition](../../user-stories/gm-set-token-hp-condition.md)
- [player-set-own-token-hp-condition](../../user-stories/player-set-own-token-hp-condition.md)
