---
title: Track the creator of drawings
created: 2026-07-29
tags: [drawing, data-model]
---

Add an author/creator field to each drawing, recording which participant created it. The
`Drawing` data model currently has no authorship tracking at all (`ID, SceneID, Kind, Points,
Color, CreatedAt`), so there's no way to tell who drew what.

This is a prerequisite for the eraser permission model: a GM can erase any drawing, but a Player
can only erase drawings they created themselves.

## Related user stories

- [gm-erase-any-drawing](../../user-stories/gm-erase-any-drawing.md)
- [player-erase-own-drawing](../../user-stories/player-erase-own-drawing.md)
