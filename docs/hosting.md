# Hosting Longtable

Guide for self-hosting a Longtable server.

## Quickstart

Longtable is a single binary that you can run just by double clicking on it.
After running it, you can visit the application in a browser at
["http://localhost:8080"](http://localhost:8080).

From here you can create a room, share your screen and get to playing!

Every room has a six-character **room code**, like `7wdbtb`. It's the end of the
room's address, and it's what other people type into **Join a room**. Rooms
aren't listed anywhere, so the code is the only way in.

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

### Players dropping in and out

When someone's connection drops, the room waits 30 seconds before showing them
as gone. Coming back inside that window changes nothing on anyone's screen, and
puts nothing in the chat log.

On a bad network, give it longer:

```sh
longtable serve -departure-grace 2m
```

A player who closes their laptop still shows as connected until the wait is up.

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

That prints every room as `CODE NAME CREATED`. A room with the code `7wdbtb` is
at `http://<your-server>:8080/r/7wdbtb`.

Room names aren't unique. If two look right, the date usually settles it — and
if it doesn't, send both and let them pick. They'll know theirs within seconds
of opening it.

### They've forgotten the GM password

A GM who can still get into the room changes it themselves, from **Manage room**.
This command is for the ones who can't.

```bash
longtable room reset-password <room-code>
```

This assigns a new randomly generated password to the room and prints it out on
the screen.

Check the code before pressing enter. There's no confirmation and no undo, and a
typo locks out a second GM who hadn't lost anything.

Send the new password some way other than the room's own chat — that's the thing
they can't reach yet. Anyone holding it can take the room as a GM.
