---
title: Player erases their own drawing
created: 2026-07-29
status: done
---

As a Player
I want to erase only the drawings I created myself
So that I can fix my own mistakes without being able to disturb anyone else's work, including the GM's

## Acceptance criteria

- [ ] A Player can erase a drawing (freehand, line, rect, or ellipse) only if they created it
- [ ] Attempting to erase a drawing created by someone else (another Player or a GM) has no effect
- [ ] Erasing removes the drawing for everyone in the room in real time
- [ ] Erased drawings are deleted server-side, not just hidden locally, so they stay gone after a reload/reconnect
