---
title: GM adds the party to a scene
created: 2026-08-14
status: done
---

As a GM
I want to keep a list of the party and add them to any scene in one action
So that I don't have to re-create the same handful of tokens by hand every time I build a new scene

## Acceptance criteria

- [ ] A GM can add a character to the party, from scratch or starting from a token on the map,
      and can change or remove one.
- [ ] Adding the party to a scene creates a fresh token for each member, copying its name, art,
      size and owner.
- [ ] Added tokens spread over free squares rather than stacking, the same as a batch created from
      the New token dialog.
- [ ] Anyone already on the scene isn't added twice.
- [ ] Only a GM can change the party or add it to a scene.

## Notes

The criteria were rewritten on 2026-10-19 when the party moved from flagged tokens into a dialog
of its own ([party-dialog](../backlog/party-dialog.md)). The first two used to read "flag or unflag
any token" and "a fresh token for each flagged token"; the design changed under them on purpose,
and every criterion above was checked against the code after the change.
