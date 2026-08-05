---
title: Room Member sees distance preview while dragging a token
created: 2026-07-29
status: incomplete
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
