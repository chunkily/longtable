# Hosting Longtable

Guide for self-hosting a Longtable server.

## Configuration

This section will document the server's config file — its location, every available setting,
and their defaults — once that feature is implemented.

## Getting a GM back into their room

Two things go wrong often enough to deserve their own section: someone loses the link to their
room, and someone forgets the GM password. Both are routine, and both land on you.

They land on you because running the server and running a game are separate jobs in Longtable. You
might be a GM on your own server, but you needn't be, and the people with rooms on it might have
no idea what a terminal is. There's deliberately no "recover my room" page in the app — anything
that handed out room links to whoever asked would rather defeat the point of rooms not being
listed — so when someone's locked out, you're the way back in.

Be nice about it. A room link is six random characters that lived in a browser tab until something
closed it, and a GM password gets chosen once, in the exciting five minutes before a first
session, then not typed again for a month. Losing them is the normal outcome, not carelessness.
Both fixes below take about ten seconds.

Both subcommands take `-db` if your database isn't at the default `longtable.db`, and the flag has
to come *before* the slug.

### They've lost the link

Ask what the room's called, or roughly when they set it up. Either will do:

```bash
longtable room list
```

That prints every room as `SLUG NAME CREATED`. The slug is the link — a room with slug `7wdbtb`
lives at `http://<your-server>:8080/r/7wdbtb`.

Room names aren't unique, so if two look right, the date usually settles it. If it doesn't, send
both and let them tell you which is theirs — they'll know from what's inside within seconds, and
that's quicker than either of you guessing.

### They've forgotten the GM password

```bash
longtable room reset-password <slug>
```

This prints a new one. You don't need the old password, and there's nothing to dig out of the
database beforehand.

Do check the slug before pressing enter, though. There's no confirmation and no undo, so a typo
here locks out a second GM who hadn't lost anything — which turns one person's small problem into
two people's larger one.

Send the new password some way other than the room's own chat, since that's the thing they can't
reach yet. Whoever holds it can take the room as a GM, so treat it about as carefully as the link
itself.

## What you can't fix from here

If a **Player** clears their browser data, they come back as a new person. There's no accounts
system behind any of this — a browser's identity in a room is a token it stored locally, and once
that's gone there's nothing to recognise them by. They'll rejoin fine, under the same name if they
like, but the room will hold two entries for them, and any tokens the old one owned will still
belong to it.

Nothing you can run fixes that, so don't go hunting for a subcommand. The GM can put it right from
inside the room in a few clicks: open each affected token, set its owner to the new entry. Worth
telling them that up front, because the natural assumption is that something is broken, and it
isn't — a duplicate in the roster is exactly what's supposed to happen given there's nobody to
match the returning browser against.
