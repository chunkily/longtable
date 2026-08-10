---
name: longtable-copy
description: How Longtable writes for the people who use it — every on-screen string (buttons, headings, descriptions, placeholders, empty states, errors, toasts, aria-labels) and every public document (README.md and docs/). Use this whenever a change adds or edits text a user or a Host will read, including a one-word button, a label you are only renaming, or a paragraph of the hosting guide. This text follows different rules from the repo's comments and planning prose, and carrying the comment register into a button or a README is the specific mistake this exists to prevent.
---

# Writing for people who use Longtable

`CLAUDE.md` says prose "uses em dashes and reads like sentences, not telegraphese". **That rule is
for comments and planning docs, and it does not leave them.** Carrying it into a button or the
README is the mistake this file exists to catch.

There are three registers in this repo, and the difference is who is reading and what they want:

| Register | Reader | Wants |
| --- | --- | --- |
| Comments, `planning/`, these skills | Whoever changes this code next year | The constraint, the failure that produced it, the reason |
| **On-screen strings** | Someone who wants to get to their game | To be understood in half a second |
| **`README.md`, `docs/`** | Someone trying to do a thing right now | The steps, and only the reasons that change what they do |

The bottom two rows are what this file covers. They differ from each other in length and not much
else: a README paragraph can be three sentences where a button is one word, but both are read by
someone who wants to finish and leave, not by someone studying the reasoning.

There is a mechanical trap behind most of the bad text here: the copy and the comment explaining it
get written in the same pass, so the text comes out as a compressed version of the reasoning just
written above it. `Rooms aren't listed anywhere, so the code is the only way in` was a comment that
wandered into a `Card.Description`. **Write the reader's text first, cold, before writing anything
about why it's there.**

## On-screen strings

Every rule below is reverse-engineered from a string the GM of this project rewrote by hand.

1. **Say what, not why.** The reason goes in a comment beside the string, where it belongs and
   where it can be as long as it needs to be.
2. **One clause.** No em dashes, no semicolons, no subordinate clause. If it needs two sentences,
   it usually needs one shorter one.
3. **Buttons are a verb, one or two words.** `Join`, not `Join room`. The card it sits in already
   said what is being joined.
4. **Don't restate the heading or the label above it.** A description under `Join a room` that
   explains what joining a room is has said nothing.
5. **Where someone is choosing, write the option in their voice.** `I have a room code`, not
   `Someone at the table sent you a room code`. They know how they got it.
6. **Give the format, not the rationale.** `Room codes are six characters like ab23ef` beats
   anything about why codes exist.
7. **Errors name what to do next**, and sit next to the field they're about rather than in a toast
   that takes itself away. See `planning/backlog/say-a-bad-code-is-bad.md`.
8. **Cut anything the interface already demonstrates.** A six-character box showing `------` has
   already said how long a code is.

Real before-and-afters, all from `web/src/routes/+page.svelte`:

| Written | Rewritten | What was wrong |
| --- | --- | --- |
| Someone at the table sent you a room code. This is where it goes. | I have a room code. | Narrating the user's situation back at them, where they are choosing |
| Rooms aren't listed anywhere, so the code is the only way in. | *(deleted)* | A comment in a `Card.Description` |
| Six characters. A link to the room works too — paste the whole thing. | Room codes are six-characters long like `ab23ef`. | Two sentences and an em dash to say one thing |
| Join room | Join | The heading above it already said room |
| Your name (GM) | Your name | The form is the GM form; the suffix was for the writer's benefit |
| Start a new table. You'll be its GM, and you'll get a code to hand out. | I'm starting a new game. | Three clauses, and the next screen already says both of the last two |

The pattern across all six: **the rewrite is shorter and states a fact; the original explains.**

The last two rows are one pair and worth reading together. They sit side by side on the welcome
screen, under `Join a room` and `Create a room`, and for a while they disagreed: one was the user's
voice in four words, the other was three clauses of mine. **A rule applied to one control and not
its neighbour is worse than not applying it at all** — the inconsistency reads as a mistake, where
either voice used consistently would have read as a choice. When a rule lands, sweep the row.

**`aria-label` and `title` count**, and are held to the same rules with one exception: they may name
what the visible label leaves to context, because a screen reader user has no surrounding card to
read from. `aria-label="Take Bob's seat"` on a button reading `Bob` is right, and so is
`aria-label="Copy room code"` on an icon.

## README.md and docs/

Same instinct, more room. A Host reading `docs/hosting.md` is mid-task — the server won't start, or
a GM has lost a password — and wants the command.

1. **Lead with the action.** The command, the step, the thing to click. Context after, if at all.
2. **One idea per sentence.** The failure here isn't length, it's stacking: a sentence that says
   what a thing is, what it's for, and where it appears has buried all three.
3. **Cut the rhetorical joins.** `which is both`, `it's the only way`, `and that's quicker than` —
   these are the seams of an argument, and a Host isn't being argued with.
4. **Reasons only where they change a decision.** "Do check the code before pressing enter, there's
   no undo" earns its place because it changes how carefully someone types. "The code has no address
   in it, so it's correct from wherever it's pasted" does not.
5. **Show, don't characterise.** A code block or a sample line beats a sentence describing what the
   output looks like.

From `docs/hosting.md`, both written and then rewritten in the same day:

| Written | Rewritten |
| --- | --- |
| Every room has a six-character **room code** — `7wdbtb`, say — which is both the last part of its address and the thing anyone else types into **Join a room** on the home page. It's the only way into a room: rooms aren't listed anywhere, for anyone. | Every room has a six-character **room code**, like `7wdbtb`. It's the end of the room's address, and it's what other people type into **Join a room**. Rooms aren't listed anywhere, so the code is the only way in. |
| That prints every room as `CODE NAME CREATED`. The code is the whole address — a room with the code `7wdbtb` lives at `http://<your-server>:8080/r/7wdbtb`, and someone who has the code can also type it straight into the box behind **Join a room** on the home page. | That prints every room as `CODE NAME CREATED`. A room with the code `7wdbtb` is at `http://<your-server>:8080/r/7wdbtb`. |

Note what survived the second one: the sample output and the worked address. Note what went: the
sentence explaining that the code can also be typed in, which the quickstart already said.

**`README.md` and `docs/` own different things** — the table at the foot of `CLAUDE.md` says which,
and the fastest way to make a doc worse is to explain something there that another file owns.

## Checking a piece of text

- Read it with the code and the comment hidden. Does it still make sense, and is any word doing
  nothing?
- Does it contain **so**, **because**, **rather than**, **which means**, **which is both**? Those
  are comment words. The clause they introduce belongs in a comment, or nowhere.
- Would you say it out loud to someone next to you? "Someone at the table sent you a room code,
  this is where it goes" is not something a person says.
- Count the clauses. On screen, more than one is a rewrite. In a doc, more than two is a full stop
  waiting to happen.

## When the rules and an edit disagree

The rules are derived from edits, so an edit always wins over a rule. If the GM rewrites something
in a way none of the above predicts, **add it to a table here and adjust the rule in the same
commit** — the examples are worth more than the rules, and this file is only as good as its
examples.

Where a rule genuinely doesn't fit, say so in the commit rather than quietly breaking it. Real
exceptions exist: the GM password hint on the create form runs three clauses and stays that way,
because it answers a question people actually have about a password they might lose, and the
alternative is a support conversation.

## This file is not an example of itself

It's written in the repo's prose register — long sentences, em dashes, reasons attached — because
it is a skill, and skills here follow `CLAUDE.md`'s house style. Don't tighten it to match the copy
it describes. The whole point is that the registers are different.
