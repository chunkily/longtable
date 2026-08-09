---
title: The token editor lost an edit and never showed its Save button
created: 2026-08-09
status: done
tags: [tokens, ui]
---

Reported from use: on a large screen the edit dialog still had a scrollbar, the trackers were
changed and a condition added, the dialog was clicked away, and the changes were gone. The Save
button had been below the fold the whole time and was never seen.

Measured before touching anything, on a 1600×1000 viewport with an empty asset library:

| | |
| --- | --- |
| Viewport | 1000px |
| Dialog | 792px, y=104…896 — **208px of screen unused** |
| Form's scroller | clientHeight **700** (`max-h-[70vh]`), scrollHeight **792** |
| Footer | y=927…961 — **below the dialog's own bottom edge** |

Section heights: Trackers 182 (≈40 of it help text), Owner 93, Size 89, Image 82 (taller with art
in the library), Name/Conditions/Visibility 55 each, footer 34, plus 112px of gaps.

## Four failures, not one

1. **The commit control scrolled away**, because `Dialog.Footer` was inside the scrolling `<form>`.
2. **The dialog scrolled when it didn't need to**: `70vh` is a fraction of the viewport, applied
   to the form, and ignored the 208px the dialog had spare.
3. **Dismissal discarded silently**, with no warning and no undo.
4. **The app teaches the opposite model three centimetres away.** Tracker values in the
   selected-token panel commit on blur. The same numbers in the dialog were staged. That is the
   mental-model failure under the other three, and the reason this reads as a bug.

## What shipped

**1 and 2 are fixed outright, and they are what would have prevented the incident.** The dialog is
capped at `calc(100dvh-2rem)` and laid out header / scrolling body / pinned footer; only the fields
scroll. Re-measured on the same viewport: dialog 880px, body 740px with **no scrollbar at all**,
footer at y=883…917, inside the dialog. The Save button is on screen the moment it opens.

**Cancel, and three ways out that mean the same thing.** `Cancel` (bottom-left, away from the
button it undoes), Escape and the X all discard — the form is reloaded from the token on every
open, so all three are simply closing without sending.

**Clicking away asks, in place of the form.** It is the ambiguous gesture — as often a misclick as
a decision — so with something typed it swaps the editor for a question with three answers:
`Back` (returns to the form with what was typed still in it), `Discard changes`, `Save changes`.
One dialog on screen at a time, which is why the editor closes rather than the question stacking
over it; the form's values live in the component, so they survive the swap. With nothing typed
there is nothing to ask and it just closes.

**Token edits are undoable.** A new history entry holds both sides — the reverse of an edit is
another edit, and `token.update` carries no history, so "what it was" is only knowable at the
moment it stops being true. Skipped on undo if the token no longer matches what this session last
set, the same rule moves follow.

**`updateToken` now drops a submit that changed nothing**, which is worth more than it sounds: an
editor left open while someone else worked on the same token would otherwise stamp its stale copy
of every field over their edit on submit, since the command carries all of them.

Three callers needed the same "are these the same token?" answer — the dialog's dirty check, the
no-op guard and the undo guard — so it is one tested module (`web/src/lib/token-fields.ts`) rather
than three hand-rolled comparisons, each free to forget the conditions array.

**A trap for whoever writes the next dialog spec**: the dismissible layer attaches its
outside-click listener a tick *after* the content paints, so a click sent the instant the dialog
appears lands before anything is listening and the dialog just stays open — indistinguishable from
dismissal being broken. Only a test can hit that window. `clickAway` in `token-edit.spec.ts`
carries the wait and the reason.

## Not done

Failure 4 is still open: the panel commits on blur and the dialog stages. The layout fix removes
the harm, and the warning covers the rest, but the inconsistency is real — making the dialog
commit as you go (no Save button, just Done, with undo as the safety net) is the version with no
failure class in it, and is its own conversation.
