---
title: Room Member switches their own viewed scene
created: 2026-08-14
status: incomplete
---

As a Room Member
I want to look at any scene in the room, not just the one everyone else is on
So that I can check a map I'm curious about, or (as a GM) prep a scene before revealing it

## Acceptance criteria

- [ ] Switching my own view doesn't change what any other Room Member sees.
- [ ] A fresh join or reconnect lands me on the room's current active scene.
- [ ] The scene list is reachable by every Room Member, not GM-only, since switching no longer
      moves the table.
- [ ] My token selection, undo history and in-flight gestures behave sanely across a switch — at
      minimum, nothing crashes or silently applies to the wrong scene.
