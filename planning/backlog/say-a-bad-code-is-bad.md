---
title: Say a room code is wrong where the code is
created: 2026-08-10
status: done
tags: [rooms, ui]
story: room-member-sees-their-own-rooms
---

Two places tell someone a room code is no good, and neither did it where they were looking.

On the home page, a malformed code raised a toast in the corner — away from the field that caused
it and away from the field that has to change to fix it, and gone again a few seconds later. Anyone
who glanced down at the keyboard to retype missed the only explanation they were given.

On the room page, a *well-formed* code with no room behind it said nothing at all. `listSeats`
404s, the catch swallowed every failure alike, and the join screen carried on: pick a role, look at
an empty seat list, say you're new, type a name, submit, fail. Five steps to learn something the
server answered in the first fifty milliseconds.

- [ ] The home page's error under the box, in danger text, marking the field
- [ ] The room page says immediately when there is no room behind the code

## What shipped

**Home page.** `codeError` state instead of `toast.error`, rendered under the input and taking the
format hint's place rather than stacking under it — the two said the same thing, and swapping keeps
the Join button from moving down the page at the moment someone is reaching for it. `role="alert"`
so it's announced, since nothing moves focus. `aria-invalid` on the input, which lights the box
through styling the base `Input` already carries, and which is the machine-readable half a toast
could never have. Typing anything clears it.

**Room page.** `ApiError` now carries `status`, and `isNotFound` is exported beside it as a guard
rather than left to callers — a bare `err.status === 404` needs the `instanceof` first, and
whoever forgets it gets `undefined === 404` and a silent false. The pre-join screen is replaced by
a "No room with that code" card, with the code repeated back for checking against whatever it was
copied from, and a link to the start.

**Only a 404 stops anything, and that distinction is the point.** The catch it replaced swallowed
every failure, and the comment above it argued for that: telling someone the room is broken when
they could simply join is worse than the silence. That reasoning still holds for a blip or a 500 —
neither says anything about whether the room exists — and both still fall through to the join form.
A 404 is the one answer that means the question has no answer.

The card replaces the join screen rather than sitting above it. Every question it asks — which side
of the table are you on, which chair is yours — has no answer here, and leaving them up invites
someone to keep answering them, which is exactly the walk this item is about.

### Found in passing

The full suite went from 109 passing to **3 passing and 106 timing out**, and none of it was these
changes. The create form's second field had been relabelled from `Your name (GM)` to `Your name`,
and `createRoom` in `e2e/fixtures/room.ts` still asked for the old spelling. Playwright matches
labels by substring, so it resolved to nothing — and `fill` on a locator that matches nothing waits
out the whole 60s test timeout rather than failing fast. Every spec creates a room, so one label
cost a 14-minute run that reported 106 unrelated failures.

Worth remembering for the shape rather than the fix: **a suite that fails almost entirely is
usually one shared helper, not a regression.** Read one `error-context.md` from `test-results/`
before reading the list of failures — the page snapshot in it named the wrong label immediately,
where the failure list said nothing but "everything is broken".
