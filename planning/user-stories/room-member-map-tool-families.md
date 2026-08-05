---
title: Room Member picks tools from a small set, not a wall of them
created: 2026-08-04
status: incomplete
---

As a Room Member
I want the map tools grouped so that only the controls for the tool I'm using are on screen
So that the toolbar stops covering the map with buttons I'm not using

## Acceptance criteria

- [ ] The permanent tool row is hand, draw, measure, fog and ping — five icons, with New token
      alongside them
- [ ] Picking a tool shows a contextual strip with that tool's variants and settings, and nothing
      belonging to any other tool
- [ ] Hand and ping show no strip at all, because neither has options
- [ ] The eraser is reached from the draw tool, not from its own top-level button
- [ ] The four area templates are reached from the measure tool
- [ ] The fog tool's two bulk actions keep readable text labels rather than becoming icons, since
      they wipe a whole scene's fog and can't be undone
- [ ] Every tool that could be reached before can still be reached, in at most two clicks
