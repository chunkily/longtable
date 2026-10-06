---
title: Show who has which token selected
created: 2026-10-06
status: open
tags: [tokens, canvas, protocol, playtest]
story: room-member-sees-who-selected-what
---

Playtest feedback: people want to see which token someone else is looking at. Selection is local to
each browser today ([token-selection-highlight](token-selection-highlight.md)) — a scoping choice,
not a principle, and that item's real requirement (two people can have different tokens selected)
still holds once selections are shared.

The relay already exists in the measuring tool: one entry per participant, broadcast, never
persisted, cleaned up when they go (`handleMeasureUpdate` in `internal/ws/hub.go`). This is the same
shape — a `selection.set { tokenId | null }` command and a `selection.changed` event — with four
differences that make it more than a copy:

1. **Selections are long-lived.** A measurement is a few seconds of drag, so a late arrival misses
   nothing; a selection can sit there all evening. The hub keeps the current selections in memory —
   not the database — and `state.sync` carries them, or a reconnect sees nobody's until each person
   clicks again.
2. **Hidden tokens.** A GM selecting a hidden monster must not send its id to a Player — that alone
   says something is there. The broadcast is filtered per recipient, the technique a hidden token's
   own broadcast uses. This is the one part that is damaging to get wrong, and it gets its own Go
   test.
3. **Presence, not sockets.** A selection clears when the room is told someone left — at the end of
   `departure_grace` — not when a socket drops, or a locking phone makes it flicker exactly as the
   badges used to. It belongs to the seat: one person on two devices is one selection, last click
   wins.
4. **Rendering.** Your own selection keeps the rotating pale-yellow ring. Everyone else's is a
   solid ring in their seat colour (`seatHex`, so the GM's is the black/white pair), offset from
   yours so the two never sit on top of each other. Several people on one token split the ring into
   arcs, one each; hovering names them.

**Everyone sees everyone, the GM included.** Keeping the GM's selection private was considered —
it stops a GM checking the dragon's HP from telegraphing a target — and decided against in favour of
the symmetric version, which a GM can also use deliberately. Hidden tokens are still filtered.

- [ ] `ws-protocol.md` gains the command and event, who may send them, and that nothing persists
- [ ] `canvas.md`'s layer table and the e2e specs' layer-index comments, if the rings go on a
      layer of their own
- [ ] Selecting a token on a scene someone else isn't viewing is harmless — ids are scene-scoped,
      and a client ignores a selection of a token it doesn't have
- [ ] A deleted token's selections are cleared for everyone
- [ ] When this ships: CLAUDE.md's "the selection is local to that browser, never synced", and a
      note on `token-selection-highlight.md` pointing here
