---
title: A welcome screen instead of an empty list
created: 2026-08-10
status: done
tags: [rooms, ui, wording]
story: room-member-sees-their-own-rooms
---

The first thing a new browser sees on `/` is a card reporting that it has no rooms, with a
one-line "Have an invite?" box under it and the create-room form under that. The two things a
newcomer can actually do are the smallest things on the page, and the largest is a statement of
what they don't have.

Replace it with a welcome screen: the two actions as large buttons of equal weight, each opening
its own step. Keep the room list for browsers that have rooms — that part works and is why
[home-page-lists-your-rooms](home-page-lists-your-rooms.md) exists — but don't draw it when it
would be empty.

Second half, same change: settle on **room code** as the public word for the six characters. It is
currently "invite link", "invite", "link", "code" and "slug" depending on where you read, and the
recovery path in `docs/hosting.md` uses two of those in the same paragraph.

- [ ] A fresh browser is offered `Join a room` and `Create a room`, and no list
- [ ] Each opens a step of its own, with a way back
- [ ] The room list still appears, above them, for a browser that has rooms
- [ ] One word — room code — across the UI, the README, the hosting guide and the CLI

## Related user stories

- [room-member-sees-their-own-rooms](../user-stories/room-member-sees-their-own-rooms.md)

## What shipped

`web/src/routes/+page.svelte` is a three-step machine — `welcome`, `join`, `create` — mirroring
the room's own join screen, which had already been through this argument and reached the same
answer: ask one question at a time, and let a wrong turn cost a click rather than a reload. The
empty-state card is gone rather than reworded. It was accurate and useless; the two buttons say
what to do instead, and on a fresh browser they are the whole page.

The room list survives untouched for anyone who has one, above the buttons rather than replacing
them — a returning GM wants their game, and someone who has been in one room can still be sent
another. `role="region" aria-label="Your rooms"` moved with it, and is now also how a spec asserts
the list *isn't* there: `toHaveCount(0)` on the landmark, rather than matching the words in the
empty card that no longer exists.

**Room code is the word now**, everywhere a person can read one: both steps of the home page, the
pre-join screen's `Room code 7wdbtb` under the room name, `README.md`, `docs/hosting.md`, and the
`longtable room list` header, which prints `CODE NAME CREATED`. `web/src/lib/invite.ts` moved to
`room-code.ts` and `parseInvite` to `parseRoomCode` — a function named after a word the UI no
longer uses is exactly the drift this item was about.

`slug` stays as the route parameter, the column and the Go identifiers. It's a URL shape, nobody
is asked to read it aloud, and renaming it would touch `[slug]` directories, every store query and
every API path for no gain a user could see. The doc comment on `room-code.ts` says so, because
the next person to notice the mismatch deserves the reason rather than the chance to "fix" it.

**The join box then went the rest of the way, in follow-up commits.** It is one large
six-character field — full width, `text-4xl` and `md:text-5xl`, monospace and letter-spaced —
rather than a small input beside a button on a mostly empty page. And `parseRoomCode` was narrowed
to match what the field now advertises: codes only, no URL and no path. It used to take the last
segment of anything slug-shaped, which meant a whole pasted link worked. That leniency served one
narrow case — someone holding a link, on the machine they mean to play on, choosing to paste it
into a text field instead of following it — and cost a parser that accepted things the field
doesn't offer. Both changes are recorded here rather than in an item of their own because they
finish the same thought: the box asks for a room code, so it should look like one and take one.

The `md:` variant on that field is load-bearing and easy to delete by accident. The base `Input`
carries `md:text-sm`; without an `md:` override of its own, the huge box shrinks to 14px at exactly
the breakpoint with the most room. `cn` resolves the pair correctly only because both are the same
variant and the same property, with the override second.

### The e2e suite had to move with it

Every spec created its room by filling three fields on the home page, so putting the form behind a
button broke all 23 of them at once. They now call `createRoom` from `e2e/fixtures/room.ts`, which
is where this should have lived before: it is the single most copied block in the suite, and this
is the second layout change in two months to have rewritten every copy.

The helper carries two waits with reasons attached. `waitForLoadState('networkidle')` before the
click, because a click that lands before hydration does nothing at all and leaves you waiting on a
form that never opened — the same race
[e2e-flakes](e2e-flakes.md) documents, in its new position. And `expect(getByLabel('Room name'))`
after it, which is a genuine improvement over what it replaced: the form appearing *proves*
hydration finished, so the three fills below it can no longer be reconciled back to empty. The
failure that item describes is now unreachable rather than merely rarer.

The comment in `e2e/run-app.mjs` about wiping the database was updated at the same time. It still
described the create form as sitting under an unbounded list, and still gave the coordinate-based
diagnosis that `e2e-flakes` had already corrected.

Verified with the full suite at 107 passed — 106 as before, plus one new spec for what a browser
that has never been anywhere is shown.
