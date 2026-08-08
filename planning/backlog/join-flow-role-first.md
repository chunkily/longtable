---
title: Ask which role you're joining as before anything else
created: 2026-08-08
status: done
tags: [onboarding, identity]
story: room-member-takes-their-seat
---

The pre-join screen shows everything at once: the seat list, a Player/GM toggle, a name box and —
once GM is picked — a password field. Whichever of the two people is reading it, most of that
screen is addressed to the other one, and the two paths share nothing but the name box.

Ask the question in the order the answers actually branch:

- [ ] First screen is the role: `Player` or `I'm the GM`, and nothing else
- [ ] GM leads to the display name and the room password, as today
- [ ] Player leads to the room's seats, with `I'm new here` as a slot on that list rather than a
      way out from under it
- [ ] `I'm new here` leads to the display name box
- [ ] Every step after the first can go back

## Traps

**The seat list is fetched, and the seats step can now be reached before it lands.** On the old
screen the list either was on the page or wasn't. Now someone can click `Player` while the request
is still in flight, and an empty list rendered at that moment is indistinguishable from a table
nobody has sat at — which is precisely how you end up on a fresh seat beside the one holding your
tokens.

**The GM seat is a role boundary, not an identity one.** It is still never on the picker
([ADR-0008](../decisions/0008-seats-and-sessions.md)); the room password is the way into it, and
that is why the role question can come first at all.

## Related user stories

- [room-member-takes-their-seat](../user-stories/room-member-takes-their-seat.md)

## What shipped

All of the shape above, as a four-step machine in `web/src/routes/r/[slug]/+page.svelte`:
`role → gm` and `role → seats → name`, with a `Back` control on every step but the first. Nothing
below the page changed — no endpoint, no protocol, no store call. `mode` and `newHere` are gone;
one `step` value now says both which screen is up and, in `handleJoin`, which of `gmLogin` and
`joinRoom` to call, so the two can't disagree.

**`I'm new here` is a dashed slot at the foot of the seat list, not a link under it.** Being new is
one of the answers to "which of these is you", and on a room nobody has joined it is the only slot
on the list — the list is never replaced by a bare name box, which is what the old screen did as
soon as there were no seats to show.

**The seats step distinguishes "still asking" from "nobody here".** `seatsLoaded` flips in a
`.finally`, so a failed request still opens the step rather than hanging on it — the list was
already best-effort, and telling someone the room is broken when they could simply join is the
worse answer. `join-flow.spec.ts` holds the `/seats` response open with `page.route` to prove the
slot is unreachable until the answer arrives; that is the one behaviour here that no amount of
clicking through by hand would have caught.

**Every e2e that joins a room needed updating, which is why the helpers exist now.** `e2e/room.ts`
gained `joinAsNewPlayer`, `takeSeat`, `joinAsGM` and `openSeatPicker`, and the fifteen specs that spelled
the old two-line join out now call them. `openSeatPicker` waits for the `I'm new here`
slot rather than for a seat or for the heading — the slot is the one control that renders whatever
comes back, and the heading isn't (it reads "Nobody has taken a seat at this table yet" on an empty
room). Specs that assert a particular seat is *missing* would otherwise pass against a list that
simply hadn't arrived, the same negative-assertion trap recorded in [e2e-flakes](e2e-flakes.md).

Found and fixed in passing: `room-sync.spec.ts` matched a room name by bare text, which has
resolved to two elements ever since every page got a real `<title>` — SvelteKit's live-region
announcer carries the title too. It is a heading match now. `.claude/launch.json` also still named
`web/e2e/run-backend.mjs`, renamed to `run-app.mjs` when the suite moved onto the shipped binary;
the entry is `app` rather than `backend` now, because it serves the frontend too.
