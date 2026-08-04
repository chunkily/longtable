---
title: Token HP / condition tracker
created: 2026-07-29
tags: [tokens, gameplay]
---

Add controls to make it easy to track damage / hp / conditions on individual tokens.

The surface to put them on now exists: [token-detail-panel](../done/token-detail-panel.md) has
shipped `token-detail-dialog.svelte`, opened from the token details section above chat, and
`token.update` to carry the fields. What's left here is the part that item deliberately didn't
touch — the columns (three labelled numeric tracker slots plus condition tags), the fields on
`store.Token`, and adding them to the dialog and the payload.

Two things from that item that bear on this one. `token.update` sends every editable field every
time and the handler edits the loaded token in place, so adding a field means adding it to the
request struct, the assignment block and the payload — miss the assignment and it silently never
saves. And the role check is currently a single GM-only gate at the top of the handler; the
`player-set-own-token-hp-condition` story needs it to become per-field, since an owner should be
able to change HP without being able to change visibility.

## What shipped

Every token carries **three numeric tracker slots**, each with a label of its own, and **any
number of condition tags**. A GM edits them on any token from the same Edit dialog as before; a
Player edits them on a token they own, from a cut-down version of that dialog with nothing else on
it. Both show in the details panel above chat and on a **card that appears when the pointer rests
on the token** — a new Konva layer, index 9, appended so the existing layer indices the e2e specs
read pixels from didn't move.

**The values are also editable in the details panel itself**
(`token-tracker-strip.svelte`), for whoever may edit the token at all. Damage changes every round
and is the last thing that should cost a dialog. Three decisions in that strip:

- **Values there, labels in the dialog.** A label is set once when a creature arrives and read all
  evening; a text box for it beside the numbers would double the width of a strip that has to fit
  next to the token's name.
- **Committed on `change`, not `input`.** Per-keystroke sending means typing "12" broadcasts a
  one-point total on the way past, and a held key floods the socket. The change event is the blur
  or the Enter — when someone has finished saying what they meant.
- **`setTokenTrackers` fills the rest of the token in from client state**, because `token.update`
  clears what it isn't told. That makes an inline edit last-write-wins against a dialog open on
  the same token elsewhere, which is the bargain the dialog already makes with itself and is only
  ever racing over fields a Player can't change anyway.

The panel shows **all three slots always**, an unset one reading as a dash; the hover card shows
**only slots carrying a number**. Not an inconsistency — the panel is read while a token is being
worked on and wants three fixed positions to scan, whereas a card floating over the art wants to
say as little as possible.

**The relationship between the two paths is the interesting part**, because it made ownership mean
something for the first time. `token.update` had one GM-only gate at the top; it now has a
per-field check, and the shape of it is worth keeping:

- A Player's update is **applied to the trackers and conditions and nothing else**. The GM-only
  fields aren't rejected — the loaded token simply keeps them, the same in-place edit that already
  protected a field the command didn't carry. Rejecting instead means diffing every echoed field
  against what's stored and calling any difference an attack, which turns a stale form into an
  error and protects nothing extra. The client sends the whole payload from both roles for the
  same reason: one method, and a narrower one would read as a GM clearing the name.
- **A hidden token is refused to a non-GM in the words of one that doesn't exist — even to its own
  owner.** A GM can prep an ambush with a Player's character, and an error separating "not yours"
  from "no such token" is how they'd find out. A *visible* token they don't own gets the plainer
  "you can only edit a token you own", which leaks nothing they can't already see.
- Deleting stayed GM-only. Being allowed to take damage on a token is a long way from being
  allowed to remove it.

Three modelling decisions that would be expensive to rediscover:

- **`value` is nullable, and null ≠ 0.** An empty slot and a creature on nought hit points are the
  two states a GM most needs told apart, and 0 is the more important of them. It's a pointer on
  the wire, in `store.Tracker`, and in the TS type; the value input is bound as a *string* rather
  than through `type="number"`'s `valueAsNumber`, because that reports NaN for both an empty box
  and — depending on the browser — a box being cleared. There are tests at the store, hub and
  vitest levels doing nothing but pinning this.
- **Labels are per token, not per room.** The story says "I can label each tracker" without saying
  where the label lives; per-token is the reading taken, because a monster's third slot is
  legendary resistances and a wizard's is spell slots, and there is no room-wide settings surface
  to hang a shared label off anyway.
- **Three slots, fixed.** `store.TrackerSlots`, normalised on every read and write, so nothing
  downstream checks the length. More than three from a client is an error rather than a silent
  truncation: it means the client disagrees about how many there are, and keeping the first three
  would drop whatever was typed last.

`token.create` carries both too, though tokens are created blank. Undoing a deletion rebuilds the
row from that payload alone, and a token coming back on full health is worse than the misclick
being undone.

The hover card shows **nothing at all** for a token with no numbers on it. Every token popping an
empty box as the pointer crossed it makes the map unusable during a fight, which is when this is
for. It has its own layer because `renderTokens` destroys the token layer wholesale on any change
to `room.tokens` — a card living there would blink out whenever anyone moved anything.

Two traps hit while writing the e2e spec, both now written up in `references/testing.md`:
**each page needs its own canvas box** (a GM's toolbar is 44px taller than a Player's, and against
a 70px grid only a 1×1 token misses — which is why `token-edit.spec.ts` shares one box and passes
anyway), and **`npx playwright test` must be run from inside `web/`** or npm resolves a different
`@playwright/test` and blames a spec file that is fine.

Left undone deliberately: no maximum on a tracker, so nothing renders as a bar — the story asked
for "a single current number per slot, no max for now". Conditions are free text rather than a
picked list of the SRD's; a fixed list is a smaller change once someone wants it.

## Related user stories

- [gm-set-token-hp-condition](../../user-stories/gm-set-token-hp-condition.md)
- [player-set-own-token-hp-condition](../../user-stories/player-set-own-token-hp-condition.md)
