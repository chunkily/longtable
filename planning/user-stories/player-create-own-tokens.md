---
title: Player creates their own tokens
created: 2026-08-04
status: done
---

As a Player
I want to put my own tokens on the map and take them off again
So that my summons, familiars and companions don't become the GM's paperwork mid-fight

## Acceptance criteria

- [ ] I can create a token from the same button a GM uses, without asking the GM
- [ ] A token I create is owned by me, without my having to choose an owner
- [ ] I can't choose whether it's hidden — that stays a GM control
- [ ] I can delete a token I own, and only one I own
- [ ] A GM can still edit and delete any token, including mine
- [ ] Everyone in the room sees it appear and disappear live, the same as a GM-made token

## Verified 2026-08-08

Every criterion holds, against `internal/ws/hub.go`'s `handleTokenCreate` and `handleTokenDelete`
and covered end to end by `web/e2e/player-tokens.spec.ts` with two browser contexts — the "GM can
still delete mine" half is `token-trackers.spec.ts`'s ownership test, which had to be rewritten:
it asserted a Player owning a token got an editor but *never* a delete, which was true right up
until this shipped.

Worth knowing beyond what the criteria asked: the fields a Player may not set are **ignored, not
refused**, matching `token.update`. A form that sent `visibility: hidden` gets a visible token and
no error. Refusing instead would mean treating every stale form as an attack, and the value is
preserved either way.
