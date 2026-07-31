---
title: Room Member measures an area of effect
created: 2026-07-29
---

As a Room Member
I want to lay an area-of-effect template on the map from a point I choose
So that I can see what an ability covers before committing to it

## Acceptance criteria

- [ ] I can choose a circle, cone, line or cube template
- [ ] The template originates from wherever I start the drag, and dragging sets its size and
      direction
- [ ] I can set a Line's width, which a single drag can't express
- [ ] I can choose whether template points snap to grid intersections, to cell centres, or not at
      all
- [ ] The template's size in feet is shown while I drag it
- [ ] The template is visible to all Room Members in real time while I'm actively measuring
- [ ] The template disappears once I finish; it isn't persisted

## Notes

An earlier criterion read "the grid squares the template currently covers are highlighted". It
was **dropped deliberately**, and it's worth knowing why before anyone adds it back: tables
disagree about which squares an area catches. Some place a sphere on a cell centre and some on an
intersection; there is a live argument about whether a 5 ft cube covers one square or four.
Highlighting squares would make this app pick a side, and pick it invisibly — the template on
screen would say one thing and the rules at the table another.

Drawing the true shape and leaving the reading to the players is what a paper cutout on a battle
map always did, and it's the only version that is right for every table. The snap-mode criterion
above is the replacement: it lets a table apply its own convention to where a template may sit,
rather than having a coverage rule applied to it.

The six area shapes in the 2024 PHB come down to four outlines on a top-down map. Sphere,
Cylinder and Emanation are all circles — a Cylinder's height is off the plane, and an Emanation
differs only in being anchored to a creature, which matters when something persists and has to
follow a token rather than while a template is being dragged. That leaves circle, cone, line and
cube.
