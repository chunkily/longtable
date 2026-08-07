---
title: Full-bleed map layout
created: 2026-08-04
status: done
tags: [ui, layout]
---

Rebuild the room page around the map. Today it's a padded document — a header row, a card holding
the canvas, a chat card beside it, and margins around all of it — which leaves a lot of the screen
unused and makes the map the smallest it could reasonably be. Instead the map fills the window and
everything else floats over it or sits in a fixed rail down the right.

Settled in a design session on 2026-08-04, working from a wireframe. The shape below is decided;
what isn't decided is called out as such.

## The map

- [ ] Canvas fills the viewport. No page padding, no card around it, no header row.
- [ ] The room name, your name and role, and connection status move into the sidebar's session
      info block (below) — there is no page header any more.
- [ ] A dropped socket still gets a banner across the top of the map, not just the status dot in
      session info. It's the one thing a Room Member must not miss, and a dot in a corner is
      missable. This is the existing `showConnectionBanner` block, relocated.

## The toolbar

Two floating clusters over the map's top-left corner, on one row: the tools, then undo / redo /
reset view. A **contextual strip** sits on the row below, carrying whatever the active tool family
needs and nothing else.

- [ ] Tool row: hand, draw, measure, fog, ping. Five icons, not eleven.
- [ ] `New token` sits alongside as its own button — it opens a dialog rather than entering a
      mode, so it isn't a tool, but it's wanted close to hand.
- [ ] **Hand** is the current no-tool state: pan and token selection. No contextual strip.
- [ ] **Ping** is a mode with no options. No contextual strip.
- [ ] **Draw** owns freehand, line, rect, ellipse *and the eraser*, plus colour and stroke width.
      The eraser moving inside the draw family is deliberate — it's the same gesture on the same
      objects.
- [ ] **Measure** owns the measuring tool and all four AoE templates, plus the size stepper and
      snap mode. This is the busiest strip and the one that decides how wide the strip must be.
- [ ] **Fog** owns reveal and hide, as icons. The two bulk actions keep their text labels —
      `Reveal all` and `Reset fog` wipe a whole scene's fog, there's no undo for fog, and as two
      adjacent unlabelled icons meaning "uncover everything" and "cover everything" they are
      exactly the pair that gets mis-hit. Text costs ~120px in a strip that has the room.
- [ ] `Scenes` and `New scene` leave the toolbar entirely — they live under the menu (below).

This absorbs [contextual-drawing-controls](contextual-drawing-controls.md) outright, and is where
[stroke-size-range-input](stroke-size-range-input.md) and [shape-fill-toggle](shape-fill-toggle.md)
land. The reason the families exist at all is that a flat row of every control was already ~176px
of opaque card over the map with three more controls queued behind it.

## The sidebar

A fixed full-height rail down the right. **Static — it does not collapse.** The panel switcher
lives inside it, so a collapsible rail would need something left behind on the map to bring it
back, and the map is still ~80% of the screen without that complication.

Top to bottom:

- [ ] **Selected token** — art, name, size, owner, and Edit/Delete for a GM. **Holds its height
      when nothing is selected**, so the rest of the rail doesn't jump every time you click empty
      map.
- [ ] **Session info** — room name, connection status, who's connected.
- [ ] **The switchable region** — chat or the initiative tracker, filling the remaining height.
- [ ] **Three icons at the foot**: chat, initiative, menu.

Width wants to be ~360–380px for chat to read well. Resizable with a remembered width was
discussed and not decided — the tracker wants less room than chat does.

## The menu

The third icon opens a menu upward: `Scenes`, `Assets`, `Manage room`, `Leave room`.

- [ ] `Assets` navigates to the existing `/r/{slug}/assets` page rather than opening a dialog.
      Folding that page into a 380px rail was considered and rejected — its upload flow stages a
      file and asks for name, credit and grid alignment, and grid alignment in particular wants
      width.
- [ ] `Leave room` is [leave-room-button](leave-room-button.md), which now has a home.
- [ ] `Manage room` is a container for GM-level room settings. It is the home for
      [room-privacy-settings](room-privacy-settings.md), the toggle in
      [token-move-ownership-lock](token-move-ownership-lock.md), and [delete-room](delete-room.md).
      A toggle for whether Players may create their own tokens
      ([player-created-tokens](player-created-tokens.md)) is a natural fourth, not yet asked for.

## Mobile

- [ ] The sidebar becomes a **bottom sheet**, keeping the three icons pinned along the bottom edge
      of the screen. A right-hand drawer sliding over the map was the alternative and was rejected:
      it buys one sidebar implementation by covering the thing you're looking at.
- [ ] The **contextual strip docks into the sheet** rather than floating over the map. Draw's
      strip is borderline at 375px and measure's doesn't fit, and horizontal scrolling on a bar
      floating over a pannable canvas is a gesture conflict. Flagged as the decision most likely
      to be revisited once there's real feedback from a table.
- [ ] The **selected-token block becomes a bar pinned above the icon bar**, shown only when
      something is selected — so it leaves the sheet entirely and the "holds its height" rule above
      applies to desktop only. Anchoring it to the token itself (flipping above/below) was the
      first idea and was rejected: it tracks a target you're dragging under your own thumb, covers
      the squares around the token, needs horizontal clamping as well as vertical flipping (at
      which point the tail stops pointing at anything), parks a delete button on a canvas you drag
      with your thumb, and needs world→screen conversion every frame because the card is DOM and
      the token is Konva. The selection ring already says *which* token; the bar only has to say
      *what*.
- [ ] The second toolbar cluster shrinks to undo alone at 375px; redo and reset view move into the
      menu.

## Land these first

Two known bugs sat squarely in this design's path, both invisible from a developer's machine — see
their items for why. **One is now fixed:**

- ~~[random-id-without-secure-context](random-id-without-secure-context.md)~~ — drawing and
  pings threw for every client on a `192.168.x.x` address, which is how every phone at the table
  connects. Fixed 2026-08-04.
- [pinch-zoom-touch-devices](pinch-zoom-touch-devices.md) — a touch device couldn't zoom the map
  at all, which would have undone most of the point of a phone-facing layout. **Fixed 2026-08-07,
  so this is no longer blocked by anything.** Worth reading its "What shipped" before starting
  here: it left `applyViewChange()` as the one place that re-renders everything sized in screen
  pixels after the view moves, and a bottom sheet that resizes the stage will need it for the same
  reason a zoom does.

## Supersedes

[chat-panel-tabs](chat-panel-tabs.md) was the smaller version of the sidebar half of this,
and is closed out unshipped. [initiative-tracker](initiative-tracker.md) is
unaffected in substance — its surface is now the panel switcher rather than a tab strip, which is
the same thing wearing different clothes.

## Related user stories

- [room-member-full-bleed-map](../user-stories/room-member-full-bleed-map.md)
- [room-member-map-tool-families](../user-stories/room-member-map-tool-families.md)
- [room-member-room-side-panel](../user-stories/room-member-room-side-panel.md)
- [room-member-mobile-room-layout](../user-stories/room-member-mobile-room-layout.md)

## What shipped

The whole shape above, desktop and mobile, in one pass. New files: `$lib/tool-family.ts` (the
`Tool` union plus the family rules, with unit tests), `map-toolbar.svelte`, `tool-strip.svelte`,
`room-menu.svelte`, `manage-room-dialog.svelte`, and `e2e/room.ts`. `+page.svelte` was rebuilt
around a `fixed inset-0` shell; the canvas container lost its `h-[70vh]` and border and now just
fills whatever box it's given.

Three decisions worth not rediscovering:

- **The active family is derived from the active tool, never stored beside it.** `familyOf` is
  total over the union and `'none'` *is* the hand, so there is always exactly one family to
  highlight and no way for the strip and the canvas to disagree. `toolForFamily` remembers what
  each family was last left on so coming back to Draw restores your shape. Neither family nor
  variant buttons toggle: there's an explicit Hand button to stop with, so a second click on the
  active family would only be a surprising way to lose the strip you were reaching for.
- **New scene went into the menu as a second entry, not inside the Scenes dialog.** The item's
  toolbar section says both live under the menu, which is what got built; the side-panel story
  lists four entries and there are five. Opening the create dialog from inside the Scenes dialog
  was tried first and left two stacked modals with two focus traps, and the list underneath isn't
  something you're still reading while naming a new scene.
- **The ruler's button is `Distance`, not `Measure`.** The family button is already Measure, and
  two controls sharing an accessible name in one view is ambiguous to a screen reader and to
  Playwright alike.

Two things the change exposed rather than caused, both fixed here:

- **A token-layer rebuild stranded `hoveredTokenId`.** The rebuild destroys every group, so the
  old one can't fire `mouseleave` and the new one fires no `mouseenter` — the hover card outlived
  the pointer and never cleared. The stage's own `mouseleave` used to paper over it, which is why
  nobody had seen it: before the map filled the window there was plenty of page outside the canvas
  to move onto. `renderTokens` now ends with `syncHoverToPointer()`, re-deriving hover from where
  the pointer actually is. `token-trackers.spec.ts` had been passing only because its "move away"
  point landed off the smaller canvas.
- **The `ResizeObserver` only called `renderGrid`.** The stage now resizes for reasons that aren't
  a window drag — the sheet opening, the strip appearing — and each moves the viewport over the
  world exactly as a pan does, so it calls `applyViewChange()` like every other view change. The
  item's own note about `applyViewChange` predicted this.

The e2e suite (79 specs) needed real work and now shares `e2e/room.ts`: a family-aware
`selectTool`, `selectToolFamily` for the fog family's one-shot buttons, menu helpers for Scenes /
New scene / Assets, and `mapGestureOrigin`. That last one is the trap most likely to catch the
next person — the toolbar floats over the map's top-left, so a drag from the true canvas origin
lands on a button, and **a spec that also probes pixels has to add `TOOLBAR_CLEARANCE_Y` back on**
because the canvas buffer still starts at its true corner. The failure mode is "expected > 0,
received 0", which reads like a broken feature rather than a mis-aimed test.

Two pieces are shape-without-substance on purpose, and both say so on screen: the initiative panel
(see [initiative-tracker](initiative-tracker.md)) and `Manage room`, whose three intended settings
are each still open items. Building the switcher and the menu now means those land as contents
rather than as a new icon nobody notices.

Not done, and deliberately: the rail is a fixed 368px, since resizable-with-a-remembered-width was
discussed at the design session and left undecided.
