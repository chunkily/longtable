# Hosting Longtable

Guide for self-hosting a Longtable server.

## Configuration

This section will document the server's config file — its location, every available setting,
and their defaults — once that feature is implemented.

## Getting a GM back into their room

A Host isn't necessarily part of any room on their own server, and there is deliberately no
recover-my-room page in the web UI: anything that handed out room links to whoever asked would
undo the point of rooms not being listed in the first place. So both recovery paths run through
the Host, at a terminal, against the database file.

Both subcommands take `-db` if the database isn't at the default `longtable.db`, and that flag has
to come *before* the slug.

### They've lost the link

Ask what the room is called, or roughly when they made it, then:

```bash
longtable room list
```

That prints every room as `SLUG NAME CREATED`. The slug is the link — a room with slug `7wdbtb`
lives at `http://<your-server>:8080/r/7wdbtb`.

Room names aren't unique, so if two match, the creation date is the tiebreaker. If several match
on both, the listing can't tell them apart and neither can you — send the candidates and let the
GM recognise the right one by what's inside.

### They've forgotten the GM password

```bash
longtable room reset-password <slug>
```

This prints a newly generated password. It doesn't need the old one and doesn't ask for
confirmation, so check the slug first: resetting the wrong room locks out a GM who hadn't lost
anything.

Send the new password by some route other than the room's own chat, which they can't reach yet.
Anyone holding it can take the room as a GM, so it deserves the same care as the link.

## What can't be recovered

A Player's identity. Sessions live in the browser with no accounts behind them, so a Player who
clears their browser data rejoins as a **new** participant even with the same display name — the
old one stays in the room's roster, and any token that was assigned to it stays assigned to it. A
GM has to reassign those by hand.

Worth knowing before you go looking for a subcommand that fixes it: there isn't one, and the
roster growing a duplicate name is the expected symptom rather than a bug.
