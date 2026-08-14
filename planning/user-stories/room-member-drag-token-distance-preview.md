---
title: Room Member sees distance preview while dragging a token
created: 2026-07-29
status: done
---

As a Room Member
I want to see a translucent ghost of a token at its original spot, a line to where I'm dragging it, and the distance in feet, while I drag it
So that I can tell how far I'm moving a token before I let go

## Acceptance criteria

- [ ] While dragging a token, a translucent "ghost" copy of it stays visible at its original position
- [ ] A line is drawn from the ghost's position to the token's current dragged position
- [ ] The line is labeled with the distance in feet (5ft per grid square, using the alternating diagonal rule), updating live as I drag
- [ ] The distance reflects where the token will actually snap to, not the raw cursor position
- [ ] The ghost and line disappear once I release the token

## Note on the second criterion

"To the token's current dragged position" was read as **to the centre of the square it would land
on**, not to the raw pointer. The two are never more than half a square apart, so the line still
visibly reaches the token under the hand — but they aren't the same point, and the choice matters
because of the fourth criterion: a line that stopped at the cursor would disagree with its own label
about which square it had reached. The distance measuring tool settled this exact question the same
way for the same reason (see the comment on `drawDistance` in `game-canvas.svelte`), and a drag
preview that drew it differently would be the odd one out.

Everything here is deliberately visible **only to whoever is dragging**. The story's "So that I can
tell how far I'm moving a token" is one person's question, and nothing in the criteria asks for the
room to see it — see the "What shipped" note on
[token-drag-distance-preview](../backlog/token-drag-distance-preview.md).
