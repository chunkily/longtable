---
title: Remap the tool shortcuts
created: 2026-10-06
status: open
tags: [toolbar, keyboard, accessibility]
story: room-member-remaps-shortcuts
---

A nice-to-have from the same playtest feedback as
[tool-keyboard-shortcuts](tool-keyboard-shortcuts.md), and dependent on it: that item's one table,
read by both the handler and the sheet, is what makes this cheap.

- [ ] Remapping happens in the shortcuts sheet itself — click a key, press the new one. No options
      page; [options-page](options-page.md) records why one was dropped, and this doesn't change
      the argument
- [ ] Stored per browser like the theme and the bold grid, never sent anywhere — a keyboard layout
      is a property of this machine, not of the room, and the people who most need this are the
      ones whose layout makes the defaults awkward
- [ ] A key already taken is refused, naming what has it, rather than silently swapped
- [ ] A reset puts every key back to its default
- [ ] A stored mapping naming a tool that no longer exists, or a key the handler now reserves, is
      dropped rather than breaking the handler
