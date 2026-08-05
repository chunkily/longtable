---
title: Fog of war hide/reset/reveal-all controls
created: 2026-07-29
status: done
tags: [scenes, gameplay]
---

Fog of war is currently reveal-only, and only cell-by-cell. `internal/store/fog.go` only has
`RevealCells` (an idempotent insert) and `ListFogCells` — there's no way to hide a cell back once
it's been revealed, no way to reset a scene's fog entirely, and no bulk reveal-all either. Scoped
to manual controls only for now — automatic vision-based fog (tokens auto-revealing based on a
sight radius) would need a walls/obstructions data model that doesn't exist at all today, and is
deliberately out of scope for this pass.

## What shipped

A GM now has four fog controls instead of one. **Hide fog** is a second painting tool alongside
Reveal fog — the same drag over the same squares, running backwards — and **Reveal all** and
**Reset fog** are one-click buttons covering or uncovering the whole scene. All four are GM-only
and all four reach every Player live. `internal/store/fog.go` gained `HideCells` (the exact
inverse of `RevealCells`, and equally idempotent) and `ClearFog`.

Decisions worth not rediscovering:

- **Reveal-all materialises every cell rather than setting a scene-level flag.** Fog's only
  representation is the set of revealed cells, so a flag would need reconciling with that set the
  first time the GM hid one square afterwards. Materialising also let it broadcast the existing
  `fog.revealed` instead of an event of its own — the client needed no new case at all, and
  the server stays the only thing that decides what cells a scene has. A client computing them
  from the scene's dimensions would have to agree exactly or drift on the next reload.
- **Hence a cap** (`maxRevealAllCells`, 40,000 — a 200×200 grid). The count is quadratic in map
  size and every cell is both a row inserted and an entry in a payload every client receives. A
  scene with no width/height or no grid is refused outright rather than silently revealing
  nothing.
- **Both fog commands are idempotent, which is what keeps the sweep simple.** A drag sends every
  cell it crossed, including ones already in the target state, so neither the tool nor the client
  has to track which squares actually needed changing.
- **Reset has no confirmation.** [gm-reset-scene-fog](../user-stories/gm-reset-scene-fog.md)
  asks for "a single action", read here as "not cell-by-cell" rather than "must not confirm" —
  but the room has no confirm dialog component to borrow, so the cheap reading won. The cost is
  real: a misclick discards a session's worth of revealed fog with no undo. Worth fixing the
  first time someone actually hits it.

The `Tool` union's `'fog'` is now `'fog-reveal'`, with `'fog-hide'` beside it; both share one
branch in `attachToolHandlers`. Note that `e2e/drawing-right-click.spec.ts` and the new
`e2e/fog-controls.spec.ts` both select fog tools by *label*, which changes when the tool goes
active — the shared `selectTool` helper can't be used for either.

Still out of scope, as originally scoped: automatic vision-based fog. It needs a walls model that
doesn't exist.

## Related user stories

- [gm-hide-fog-cell](../user-stories/gm-hide-fog-cell.md)
- [gm-reset-scene-fog](../user-stories/gm-reset-scene-fog.md)
- [gm-reveal-entire-scene-fog](../user-stories/gm-reveal-entire-scene-fog.md)
