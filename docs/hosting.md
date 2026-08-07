# Hosting Longtable

Guide for self-hosting a Longtable server.

## Configuration

This section will document the server's config file — its location, every available setting,
and their defaults — once that feature is implemented.

## Getting a GM back into their room

Longtable was designed to be hosted and administrated by people who aren't necessarily the GMs of every room. If someone loses GM access to a room because they lost the link or forgot the password, only the host can help recover access.

Two recovery scenarios are documented here for hosts to help GMs that don't have access to the hosting machine, and each should take less than a minute to do.

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

This assigns a new randomly generated password to the room and prints it out on the screen.

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
