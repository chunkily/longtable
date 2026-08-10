# Hosting Longtable

Guide for self-hosting a Longtable server.

## Quickstart

Longtable is a single binary that you can run just by double clicking on it.
After running it, you can visit the application in a browser at
["http://localhost:8080"](http://localhost:8080).

From here you can create a room, share your screen and get to playing!

Every room has a six-character **room code** — `7wdbtb`, say — which is both the
last part of its address and the thing anyone else types into **Join a room** on
the home page. It's the only way into a room: rooms aren't listed anywhere, for
anyone.

### Letting others join

The application can also be accessed by devices on the same network so they can
interact with a table that updates in real time. Other players on the same
network can access the application at "http://\<your-address-here\>:8080". If
you are unsure of your own address, the application prints out the URLs it
thinks are reachable.

For example if you see something like

```sh
INFO longtable: listening addr=:8080 db=longtable.db assets=longtable-assets
INFO longtable: reachable at url=http://192.168.1.23:8080 interface=Ethernet
```

Then other players may be able to get to the main page at
[http://192.168.1.23:8080](http://192.168.1.23:8080).

Note that if nothing is printed, the machine may have no network address to
share: check it's actually on the network.

## Advanced Configuration

This section will document the server's config file — its location, every
available setting, and their defaults — once that feature is implemented.

## Getting a GM back into their room

Longtable was designed to be hosted and administrated by people who aren't
necessarily the GMs of every room. If someone loses GM access to a room because
they lost the room code or forgot the password, only the host can help recover
access.

Two recovery scenarios are documented here for hosts to help GMs that don't have
access to the hosting machine, and each should take less than a minute to do.
You will need to run the commands in a terminal!

### They've lost the room code

Ask what the room's called, or roughly when they set it up. Either will do:

```bash
longtable room list
```

That prints every room as `CODE NAME CREATED`. The code is the whole address —
a room with the code `7wdbtb` lives at `http://<your-server>:8080/r/7wdbtb`, and
someone who has the code can also type it straight into the box behind **Join a
room** on the home page.

Room names aren't unique, so if two look right, the date usually settles it. If
it doesn't, send both and let them tell you which is theirs — they'll know from
what's inside within seconds, and that's quicker than either of you guessing.

### They've forgotten the GM password

```bash
longtable room reset-password <room-code>
```

This assigns a new randomly generated password to the room and prints it out on
the screen.

Do check the code before pressing enter, though. There's no confirmation and no
undo, so a typo here locks out a second GM who hadn't lost anything — which
turns one person's small problem into two people's larger one.

Send the new password some way other than the room's own chat, since that's the
thing they can't reach yet. Whoever holds it can take the room as a GM, so treat
it about as carefully as the room code itself.
