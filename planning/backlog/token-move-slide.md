---
title: Tokens slide to their new square
created: 2026-08-02
status: done
tags: [tokens, ui]
---

A token someone else moved jumped straight to its new square, so a move you weren't watching for
was easy to miss entirely — and hard to attribute, since nothing showed which token had gone
where. Asked for directly rather than coming off the backlog, with the observation that it could
be done client-side. It could: the server already sends the destination and always did.

## What shipped

A ~0.22s eased slide from the square a token left to the one it arrived at, on every client that
learns about the move from the broadcast. `prefers-reduced-motion` gets the old instant jump.

The thing worth knowing before touching this: **it needed no change to how tokens render.**
`renderTokens` destroys and rebuilds every group on any change to `room.tokens`, so there is
never a node left to animate — which looks like it forces diffing, the thing
`references/canvas.md` argues against everywhere else. Instead the renderer remembers where it
last *drew* each token and builds the new group at that position before tweening it to the
current one. Wholesale rebuild intact.

Four cases that each need their own handling, and each of which is a visible bug if missed:

- **The person doing the dragging must not see it slide.** They have already watched the token
  travel under their own pointer; the echo would otherwise find it remembered at the square it
  left and rubber-band it back before sliding forward again. `dragend` records the snapped
  position immediately, so the broadcast finds nothing to animate.
- **A re-render mid-slide** — anyone moving any other token — would restart the slide from a
  resting position the token had already left, snapping it backwards. Each group's *current*
  position is read before the rebuild and used as the start instead.
- **A scene change** clears the remembered positions, or every token on the new map slides in
  from wherever some unrelated token stood on the old one.
- **A token leaving the scene** has its entry pruned, so an undone deletion — which restores the
  same id — doesn't slide in from where the token was before it was deleted.

Duration is fixed rather than scaled by distance. A twenty-square move taking twenty times as
long reads as a different kind of event; what this is for is letting the eye follow which token
moved and roughly where from, and a fifth of a second does that at any distance.

Tweens run on the token layer rather than a layer of their own, unlike the selection ring. The
ring's `Konva.Animation` runs for as long as something is selected; these last a fifth of a
second and stop, so they don't hold the layer redrawing.

One testing note. Catching a one-shot 220ms animation by polling from the test doesn't work — it
passed alone and failed under a loaded four-worker run, because `expect.poll` that misses the
window never gets another chance. `token-slide.spec.ts` samples the halfway square on every
animation frame *inside the page* instead. The probe was checked against a build with the
animation disabled, and fails there, which is the only way to know it measures anything.
