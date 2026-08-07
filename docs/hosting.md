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

## What a Player can fix themselves

A **Player** who clears their browser data, or turns up on a different device, doesn't need you.

Identity in a room is a *seat* — a chair at the table that outlives any particular browser. A
device proves it holds a seat with a token stored locally, but the seat is the durable part, and
it's what tokens are owned by. So a Player opening the room on a device that doesn't remember them
gets a list of the room's seats, takes their own back, and finds their tokens and their name
exactly where they left them. No password, no approval, nothing for you to run.

This is also why the same person on a phone and a laptop is one person rather than two: both
devices sit in the same seat, and the room shows one entry for them.

Two things worth telling a GM up front:

- **Anyone with the room link can take any seat.** That's deliberate: getting to the list at all
  means having been given the link, and the GM watches the roster in real time. Longtable trusts
  the people at the table and guards the way in, rather than putting a lock on every chair. The
  GM's own seat is the exception and needs the room password — which is why losing that password
  is the one thing they still need you for.
- **Seats build up over a campaign.** A GM can add one before a new player arrives, and remove one
  that's finished with, from `Manage room` inside the room. Removing a seat signs out every device
  on it and leaves anything it owned belonging to nobody.

There's still no accounts system behind any of this, and a seat isn't one: it's scoped to a single
room, carries no credential, and says nothing about who someone is anywhere else. It only
remembers which chair they sat in.
