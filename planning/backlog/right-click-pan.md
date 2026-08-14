---
title: Right-click drags the map, whatever tool is in hand
created: 2026-08-14
status: done
tags: [canvas, ui]
story: room-member-right-click-pan
---

Panning is a left-drag on an empty stage, which means it only exists in the Hand tool:
`attachToolHandlers` sets `stage.draggable(!isActive)` because a tool and a pan both open on a
left press and only one of them can own it. So moving the map while measuring a spell's range, or
part-way through sketching a corridor, costs a trip back to the toolbar and another one to pick
the tool up again — during which the thing you were pointing at has usually stopped being
interesting.

The right button is free. [no-draw-on-right-click](no-draw-on-right-click.md) deliberately made it
inert everywhere on the map — its story says "so that I can use the right mouse button for other
purposes" — and this is that purpose. A button no tool wants is exactly the button a pan can have
in every mode.

## Work

- [ ] Right-drag pans the stage, with any tool active and with none
- [ ] Suppress the browser context menu over the map, or every pan ends with a menu on top of it
- [ ] Decide whether the middle button joins in — it is the other convention for this
- [ ] Keep the arithmetic in its own module with unit tests, alongside `pinch.ts`

## Traps

**Do not read the pointer with `getRelativePointerPosition()`.** It is the pointer put through the
inverse of the stage transform, so feeding it back into that same transform makes each frame's
delta depend on the translation the previous frame set. The map accelerates away under the hand
rather than following it.

**A pan is not a zoom, and shouldn't pay for one.** `applyViewChange()` exists because a scale
change alters what a screen pixel means, so everything authored in screen pixels has to be
re-rendered. A pan changes only the translation: the overlays keep their size and ride along on
their own layers, and the grid is the one thing that needs rebuilding because it is generated for
the visible region. Konva's own `dragmove` handler already calls `renderGrid()` alone, and this
should match it.

**A right press during a left-button gesture must not disturb the gesture.** That is already the
rule for right-clicks mid-stroke, and `isPrimaryPointer` keeps the tool's own handlers off the
right button at both ends.

## Related user stories

- [room-member-right-click-pan](../user-stories/room-member-right-click-pan.md)

## What shipped

Right- and middle-dragging move the map in every tool, including none. `web/src/lib/pan.ts` holds
the arithmetic with unit tests; `handlePanStart`/`handlePanMove`/`handlePanEnd` in
`game-canvas.svelte` are the thin part, and `web/e2e/map-pan.spec.ts` drives the gesture for real.

**The handlers are `.pan`-namespaced and bound once in `onMount`, not in `attachToolHandlers`** —
the same decision the pinch handlers record, and for the same reason. That function tears down
every `.tool` handler on each tool change, so binding a pan there would hand it back only in the
modes that need it least.

**Konva ships `dragButtons: [0, 1]`, and it had to be narrowed to `[0]`.** Out of the box the
*middle* button drags any draggable node, which was never asked for and made the middle-button pan
impossible: a middle press on bare map ran Konva's stage drag alongside the new handler and the map
travelled at twice the speed of the hand, while a middle press on a token picked the token up.
Narrowing it is a one-line global set before the stage exists, and it also quietly fixes
middle-dragging a token, which nobody had noticed was possible.

**The movement lock's `mousedown.lock` guard had to learn about the button.** A token a Player may
not move swallows the press outright (`e.cancelBubble = true`) so that Konva doesn't hand the
gesture to the stage drag and pan the scene — see
[token-move-ownership-lock](token-move-ownership-lock.md), which records why. Left as it was, that
swallowed the *right* button too, and locked tokens became holes in the map a pan couldn't start
from. It now checks `isPrimaryPointer` first, which is the same helper the tools use and keeps its
original purpose intact.

**The context menu is suppressed for the whole map, not only after a drag that travelled.** A
menu that appears on some right-clicks and not others is harder to explain than one that never
appears, and on this map a right-click means pan. The cost is named rather than hidden: a
right-click menu on a token, if it is ever wanted, replaces this rather than sharing the button
with it.

It is a plain `addEventListener` on the component's own container rather than
`stage.on('contextmenu')`. Konva routes that event through `getIntersection` before firing it, so
a Konva handler runs only once hit-testing has agreed on a target — a dependency this has no use
for, since suppressing a browser default is a property of a region of the page rather than of
whatever shape is under the pointer.

### The failure that wasn't in the app, and cost two rounds

The e2e test for that suppression failed twice while the code under it was correct, and both
times the evidence was a bare `defaultPrevented: false` — which reads exactly like the app not
having tried. It was the **`table` fixture leaving the Scenes dialog open over the map**: making a
scene is a mode of that dialog, so creating one returns to the list rather than closing anything,
and `expect(canvas).toBeVisible()` says nothing about what is on top of the canvas. The right-click
landed on a scene list item. Playwright's own page snapshot in `test-results/` is what finally
said so, after two rounds of reasoning about Konva's event plumbing that were beside the point.

Two things came out of it:

- **The test asserts the event's `target` and `cancelable`, not just `prevented`.** A test for an
  absence has to prove it was looking at the right thing, or it fails identically whether the app
  is wrong or the pointer never arrived — and `preventDefault()` is a silent no-op on an
  uncancelable event, which is the same false negative from the other direction. `target` is what
  finally named the scene list item.
- **The spec dismisses the dialog itself, with Escape.** Closing it in the `table` fixture instead
  is the right shape of fix — every spec built on it would then get a map with nothing over it,
  rather than passing by luck because its first gesture lands outside the dialog — and it was tried
  and reverted. `getByRole('button', { name: 'Close' })` resolves against several buttons sharing
  that name, picked one belonging to an already-closed dialog, and spun on
  `element was detached from the DOM, retrying` until all 30 tests using the fixture timed out.
  Doing it properly wants a locator scoped to the Scenes dialog and a settled state to click in,
  which is its own change with the whole suite run behind it rather than a rider on this one. The
  hazard is written down in `references/testing.md` in the meantime.

### Decisions made along the way

- **The pan runs *during* a left-button gesture rather than being refused during one**, which was
  the first version and the wrong call. Holding the right button as well and shoving the map is
  how a ruler or a rectangle gets dragged past the edge of the screen, and that is a real thing
  people do with a spell's range. It needs no special handling, which is the pleasing part: a pan
  moves the stage by exactly the distance the pointer moved, so the *world* point under the cursor
  doesn't change while both buttons are down — the far end stays anchored and the map slides
  underneath it. Retracting the gesture the way a second finger does for a pinch was the
  alternative, and it throws away work the pointer is still in the middle of; a pinch has no
  choice, a spare button does.
- **The `.pan` handlers have to be bound before the `.tool` ones**, which they are, since Konva
  fires listeners in registration order and `onMount` runs before the effect that binds tools. The
  tool's mousemove reads `getRelativePointerPosition()` and has to see the translation this
  frame's pan already applied, or the far end lags the map by a frame.
- **`handlePanEnd` ignores a left release.** That release belongs to the gesture underneath;
  ending the pan on it would drop the map half-way through a shove still being made.
- **`button` and `buttons` number the same buttons differently** — in the bitmask 1 is *left*,
  not middle — which is why `isPanButton` says which it takes and has a test that says so.
- **Middle-drag was included.** It is the other convention for this, it costs one extra value in
  `isPanButton`, and its autoscroll on a full-bleed canvas was never wanted.
