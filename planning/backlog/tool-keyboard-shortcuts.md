---
title: Keyboard shortcuts for the tools, and a way to find them
created: 2026-10-06
status: open
tags: [toolbar, keyboard, accessibility, playtest]
story: room-member-switches-tools-from-the-keyboard
---

Playtest feedback framed this as discoverability — "which keys switch tools?" — but the honest
answer today is none. The only keys the room answers are undo and redo (`handleKeydown` in
`web/src/routes/r/[slug]/+page.svelte`). People arriving from Roll20, Foundry or Figma expect
letters and found nothing to discover. So: build them, and make them findable.

A starting table — the letters are this item's to settle:

| Key | Does |
| --- | --- |
| `H` | Hand |
| `D` | Draw; again cycles freehand → line → rect → ellipse |
| `E` | Eraser |
| `M` | Measure; again cycles distance → circle → cone → line → cube |
| `P` | Ping |
| `F` | Fog (GM only; nothing for a Player) |
| `N` | New token |
| `Esc` | Deselect the token, then back to Hand |
| `?` | Show the shortcuts |

- [ ] **The undo guard, extended:** nothing fires while focus is in anything with a text cursor,
      or with Ctrl/Cmd/Alt held, so browser and OS shortcuts are untouched
- [ ] Matched on `event.key`, not `event.code` — on AZERTY `D` is the key with D printed on it,
      which is what someone reading the list will press
- [ ] Ignored mid-gesture, so a tool can't change halfway through a stroke or a measurement
- [ ] **One module owns the table**, read by both the handler and the sheet — that is the whole
      precondition for [remap-tool-shortcuts](remap-tool-shortcuts.md) being cheap later
- [ ] Every toolbar button's tooltip carries its key, and its `aria-keyshortcuts` says the same
- [ ] A shortcuts sheet — a small dialog listing every key, undo and redo included — opened by `?`
      and from a room-menu entry. The entry is hidden on a coarse pointer, where there is no
      keyboard to use it with
- [ ] Every string through `longtable-copy` (the tooltip format, the sheet's heading, the menu entry)

Not this: remapping. See [remap-tool-shortcuts](remap-tool-shortcuts.md).
