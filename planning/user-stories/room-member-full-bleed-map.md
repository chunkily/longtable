---
title: Room Member sees the map fill the screen
created: 2026-08-04
---

As a Room Member
I want the map to take up essentially the whole window
So that I can see more of the battlefield without panning, and the screen isn't mostly padding

## Acceptance criteria

- [ ] The canvas fills the viewport — no page margins, no card around it, no header row above it
- [ ] The room name, my display name and role, and the connection status are still visible
      somewhere without opening anything
- [ ] When the socket drops I get a banner across the top of the map, not only a status indicator
      tucked into a corner
- [ ] Nothing that floats over the map is large enough to make a corner of it unusable for
      dragging a token or drawing
