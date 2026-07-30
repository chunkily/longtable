---
title: Area-of-effect measuring tool
created: 2026-07-29
tags: [tools, map, gameplay]
story: room-member-measure-aoe
---

Measuring tool for area of effects (cones / spheres) where the affected area originates from the current mouse location.

The ephemeral broadcast this needs already exists: [measuring-tool-distance](../done/measuring-tool-distance.md)
shipped `measure.update`/`measure.end`, keyed by participant and cleaned up on disconnect, plus
a canvas layer of its own for anything that only lives for the length of a drag. A template
carries a shape and a size rather than two endpoints, so the payload grows — but the lifecycle,
the throttling and the echo handling are all the same problem, already solved.
