---
title: Show and copy the room code from inside a room
created: 2026-08-10
status: done
tags: [rooms, ui]
story: room-member-share-room-code
---

There is no way to share a room from inside it. A GM asked "what's the code?" has to read it off
the address bar, and on a phone that means leaving the app. The session-info block in the rail
(`web/src/routes/r/[slug]/+page.svelte`, the `sessionInfo` snippet) already holds the room name,
who you are and the socket status, and is the natural place for one more line.

- [ ] The code on screen in the rail, selectable, for both roles
- [ ] A copy button beside it, with a confirmation
- [ ] A clipboard helper with a fallback, because `navigator.clipboard` is undefined on a LAN
      address — mirror `random-id.ts`, which exists for exactly this reason
- [ ] Unit tests for the fallback chain, and an e2e with the clipboard taken away

**Copy the code, not a link.** This is the decision most likely to be quietly reversed by whoever
touches it next, so: `window.location.origin` is where a link would have to come from, and for the
GM — the person most likely to press this — that origin is `http://localhost:8080`. They'd paste a
link to their own machine into the group chat and every Player would get nothing. The code has no
address in it, so it's correct from wherever it's pasted and whoever reads it. A link-copy could
come later from the `internal/lanurl` data the server already computes at startup, behind an
endpoint; it isn't free, and it isn't this item.

## Related user stories

- [room-member-share-room-code](../user-stories/room-member-share-room-code.md)

## What shipped

The session-info block in the rail now carries `room code h9frfp` in a monospace chip with
`select-all` on it, plus one line: *"To invite someone, send them this code — or the whole address
from your browser's bar."* Both roles see it. That is the whole feature.

**The three middle checkboxes above did not ship, and the reason is the useful part of this file.**
A copy button, a `copy-text.ts` helper with a `navigator.clipboard` → `document.execCommand`
fallback chain, seven unit tests and three e2e specs were all written and all passing —
including an e2e that removed `navigator.clipboard` the way `insecure-context.spec.ts` removes
`crypto.randomUUID`, and one that read the clipboard back to prove the content. Then the whole lot
was deleted before the commit.

**The tests passing was the problem, not the evidence.** `navigator.clipboard` is defined only in
a secure context. Playwright drives localhost. So the "secure" test exercised the path that only
the GM ever takes, and the "insecure" test could only assert that the fallback *reported* success
— with no clipboard API left to read the result back from, which is precisely the situation it was
simulating. Both were honest and neither could tell you whether a Player's phone, on
`http://192.168.x.x:8080`, would actually end up with six characters on its clipboard.
`document.execCommand('copy')` is deprecated, patchy across mobile browsers, and refused outright
in some. So the realistic outcome was a control that worked for the one person who least needed it
and failed quietly for the audience it was built for — the same asymmetry that hid the
`crypto.randomUUID` bug for as long as it did, arrived at deliberately this time.

Driving it by hand in a real browser is what settled it, and is worth repeating before adding any
clipboard control here. A synthetic `.click()` failed both paths and fell through to the error
toast; a trusted click resolved `writeText` in 64ms. That gap is the feature's whole reliability
story, and no test in this repo can see it — Playwright's clicks are trusted, so the suite only
ever exercises the good case.

**The address bar is the fallback that already works**, on every device at the table, without a
permission model. So the room points at it instead. Static text is worse UX than a working button
and better UX than a button that might not answer, which is the trade this item is recording.

If someone picks this up again, the bar to clear is a real phone on a real LAN address — not a
green suite. `git log` has the deleted implementation if it's wanted as a starting point.

### Found in passing

`planning/roles.md` still described reaching a room "by being given its link", from before the
room-code wording landed. Corrected in the same commit.
