---
title: Panning is dead while a token is held, and a stray click drops the token short
created: 2026-08-14
status: done
tags: [canvas, tokens, bug]
---

Reported as one bug — "right-click to pan doesn't work when a token is being dragged" — which
turned out to be two, the second considerably worse than the one that was noticed.

Both are Konva's drag machinery rather than anything in this repo's logic, and neither shows up
without a token actually in hand, which is why [right-click-pan](right-click-pan.md) shipped
without either being caught: every test it brought used a tool or bare map.

## 1. The pan goes deaf (the reported one)

`Stage._pointermove` returns early while `Konva.isDragging()` is true, unless `hitOnDragEnabled` —
off by default, and turning it on would put hit-testing back on every frame of every drag, which is
the cost that flag exists to avoid. The pan was bound with `stage.on('mousemove.pan', …)`, so from
the moment a token was picked up it received nothing. The *press* still registered, so the cursor
even changed to `grabbing`; not one move followed.

Worth noticing that this is exactly backwards from when it's wanted. The reason to shove the map
mid-drag is that the square you're aiming for is off screen — which is *why* you are still holding
the token.

## 2. Any second-button release ends the drag (the one nobody reported)

`DD._endDragBefore` is registered on `window` for `mouseup` and never looks at which button came
up. So pressing and releasing the right button while holding a token ended the token's drag
outright: it stopped following the cursor, and the eventual left release committed it wherever it
had frozen — several squares short of where it was dropped, silently, with nothing on screen to say
anything had gone wrong.

This was already true before the pan feature existed, and it isn't limited to a deliberate
right-click: a mouse that fires a stray middle-click does the same thing. It is the more dangerous
of the two by some distance, because the visible failure is "the GM mis-dropped a token" rather
than "the app is broken", and the token *looks* placed.

## What shipped

**The pan handlers are capture-phase DOM listeners on the container.** `mousedown`, `mousemove` and
`mouseup` via `container.addEventListener(…, true)` rather than `stage.on(…)`; the DOM knows
nothing about Konva's drag state. Capture phase specifically, because Konva binds its own listeners
on `stage.content`, a child of the container — capture runs outer-to-inner, so these land ahead of
everything Konva dispatches, which preserves the ordering the pan-under-a-tool-gesture behaviour
needs (the tool reads `getRelativePointerPosition()` and has to see this frame's translation). That
ordering used to come from being registered first on the stage; it now comes from the phase.

They call `stage.setPointersPositions(e)` themselves, since they run before Konva does it for that
event and `getPointerPosition()` would otherwise answer with the previous one.

**A `dragend` fired while the primary button is still held re-arms the drag.** The token group's
handler checks `e.evt.buttons & 1` — the live mask of what is down *now*, which is the actual
question; `button`, the one that changed, can't tell a right-release-mid-drag from a
right-release-after-drop — and calls `group.startDrag()` instead of committing.

That re-arm works because of *where* Konva fires the event, which is worth not rediscovering:
`DD._endDragAfter` fires `dragend` and only **then** deletes the drag element for anything whose
status is no longer `dragging`. `startDrag()` inside the handler finds the element still present,
flips it back to `dragging` before that check runs, and keeps its **original offset** — so the
token doesn't jump, and the gesture continues as though nothing happened. Recreating the drag from
scratch would have needed the offset recomputed and would jump on any pointer movement between the
two events.

`startTokenDragPreview` guards against the second `dragstart` this produces
(`if (tokenDrag?.token.id === token.id) return`), or the overlay would blink and rebuild mid-drag.

**Two e2e cases in `map-pan.spec.ts`**, both confirmed to fail against the unfixed canvas before
being kept — the pan one uses a committed stroke as its marker rather than the token, since the
dragged token follows the pointer whether the camera moved or not and can therefore say nothing
about whether it did.

## Related user stories

- [room-member-right-click-pan](../user-stories/room-member-right-click-pan.md) — its "with any
  tool active" criterion was only ever true away from a token drag; it is true generally now.
