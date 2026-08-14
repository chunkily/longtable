---
title: Shape fill toggle
created: 2026-07-29
status: done
tags: [drawing, tools]
story: room-member-shape-fill-toggle
---

Add a toggle to allow setting a fill on rectangle and ellipse shapes.

Lands in the draw family's contextual strip, and only when rect or ellipse is the chosen shape —
see [full-bleed-map-layout](full-bleed-map-layout.md). That strip is the mechanism
[contextual-drawing-controls](contextual-drawing-controls.md) was asking for, so this no longer
needs to solve "where does a control live that only applies to two of the four shapes" on its own.

**The strip now exists** ([full-bleed-map-layout](full-bleed-map-layout.md) shipped 2026-08-07):
`web/src/lib/components/tool-strip.svelte`, `family === 'draw'` branch. The measure strip beside
it already does the "only while the right variant is chosen" trick twice — snap mode appears once
a template is picked, and line width only for `template-line` — so copy one of those rather than
inventing the pattern. Note the strip is rendered twice, floating on a desktop and docked in the
mobile sheet, from the one component with bound props, so a control added once turns up in both.

## What shipped

A `Fill` toggle on the draw strip, shown only for Rectangle and Ellipse, off by default. The
`filled` flag is stored, broadcast and re-rendered on reload, so a shaded shape looks the same to
everyone and survives a refresh.

Decisions worth not rediscovering:

- **The fill is translucent (`FILL_ALPHA = 0.3`) while the stroke stays solid**, which is why it's
  an `rgba()` colour rather than Konva's `opacity` — that would fade the outline too and make the
  shape look like a preview of itself. A drawing sits on map art people are still reading, so a
  shaded room should say "this bit" without hiding the furniture in it. Stronger than the area
  templates' 0.18, which are transient overlays; a persistent mark reads as an accident at that
  weight. The conversion lives in `web/src/lib/drawing-fill.ts` with tests, because a mis-parsed
  channel gives a plausible colour — just not the one on the swatch that was clicked.
- **A fill on a line or a freehand stroke is dropped, not refused.** Konva would close the path and
  shade whatever the stroke looped around. `canFill` in `hub.go` decides it, and
  `RoomClient.createDrawing` applies the same rule locally — otherwise a shape switched from rect
  to line with Fill still on renders filled for the person drawing it and hollow for everyone else.
- **`shapeFilled` is read straight from the prop at event time**, like `strokeColor` and unlike
  `snapMode`. `snapMode` needs a local because `placeOrigin` closes over a plain const; these are
  prop getters and are always current. The e2e case toggles Fill *without* reselecting the tool,
  which is the assertion that would catch a change to that.
- `isFilled` in `drawing-hit.ts` was already a stub waiting for this, and now reads the field. It
  is deliberately still a function rather than an inlined field read, so hit-testing keeps asking
  one place.

### The schema grew a mechanism

This is the first column ever added to a table that already exists, and it needed one:
`CREATE TABLE IF NOT EXISTS` does nothing to a table that is already there, so editing the
definition alone leaves every existing database without the column and every query naming it
failing with "no such column". That is worse than the data loss this repo has knowingly accepted
elsewhere (the `fog_cell` DROP) — it isn't one feature starting empty, it's every query touching
`drawing` erroring.

`store.addMissingColumns` runs after `createTables` and `ALTER TABLE`s anything in `addedColumns`
that `PRAGMA table_info` says is missing. It has a test that seeds the *old* schema by hand and
checks both that the column arrives and that a pre-existing row reads as unfilled — a fresh test
database never reaches that path, so without it the migration is untested by construction.

Each entry has to be something SQLite can add in place. A changed CHECK or a dropped column is a
table rebuild and wants a real migration story rather than another row in that list.

## Related user stories

- [room-member-shape-fill-toggle](../user-stories/room-member-shape-fill-toggle.md)
