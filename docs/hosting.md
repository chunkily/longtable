# Hosting Longtable

Guide for self-hosting a Longtable server.

## Quickstart

Longtable is a single binary that you can run just by double clicking on it.
After running it, you can visit the application in a browser at
["http://localhost:8080"](http://localhost:8080).

From here you can create a room, share your screen and get to playing!

Run `longtable -h` in a terminal to see everything else it can do.

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

## The admin page

Longtable starts a second, separate address for you: the admin page. When it
starts it prints a link with a token in it, like

```sh
INFO longtable: set up at url="http://127.0.0.1:8081/?token=k3f9…"
```

Open that and you're in. It lists every setting with what it does, and saving
tells you if a change needs a restart. Everything on it is also available from
the command line — see **Settings** below.

If the line has scrolled away, or the window closed, ask for it again:

```bash
longtable admin
```

**The admin page is only reachable from the machine Longtable runs on.** It
listens on `127.0.0.1`, which your network can't route to, so nobody at your
table can open it even if they know the address. The players' address is the
other one, and that's the only one they need.

To get in from another machine, forward the port over SSH rather than moving
the address:

```bash
ssh -L 8081:localhost:8081 your-server
```

That gets you the admin page over an encrypted connection, which moving it
wouldn't.

### If you shared the link by mistake

```bash
longtable admin -new
```

That replaces the token straight away, on a server that's still running.
Whatever link you shared stops working, and so does any browser already
signed in.

### Turning the admin page off

If you'd rather not have it at all, give it no address:

```bash
longtable config set admin_addr ""
```

Nothing listens, and Longtable says so when it starts. Everything the admin page
would have done is still available from the command line — `longtable config
list` and `longtable config set` are the same settings.

To bring it back:

```bash
longtable config unset admin_addr
```

### What the token does and doesn't protect

The token stops someone who can reach the admin address from opening it.
Longtable serves plain HTTP with no encryption, so the token is readable by
anyone who can watch the connection — which on `127.0.0.1` is nobody, because
the connection never leaves your machine.

That changes if you move `admin_addr` off `127.0.0.1`. Then the page is on your
network, the token crosses it in the clear, and anyone who reads it once can get
in. Longtable says so when you make that change.

## Settings

Longtable keeps its settings in its database, next to your rooms. Change one
like this:

```bash
longtable config set departure_grace 2m
```

To see them all, with what each one does and what it's set to:

```bash
longtable config list
```

| Setting                     | Default              | What it does                                                                   |
| --------------------------- | -------------------- | ------------------------------------------------------------------------------ |
| `addr`                      | `":8080"`            | The address to listen on. `"127.0.0.1:8080"` keeps the server to this machine. |
| `admin_addr`                | `"127.0.0.1:8081"`   | Where the admin page listens. Only this machine can reach it. Empty turns it off. |
| `assets`                    | `"longtable-assets"` | The directory uploaded maps and token art go in.                               |
| `departure_grace`           | `"30s"`              | How long someone may be disconnected before the room is told they left.        |
| `uploads.max_asset_size`    | `"25 MiB"`           | The largest a single map or token upload may be.                               |
| `uploads.max_total_storage` | `"10 GiB"`           | The most this server will ever store, across every room combined.              |

Changes take effect when you restart the server. Longtable says so when you
make one.

A setting Longtable doesn't recognise is refused, and it names the one you
probably meant. A value it can't use is refused as you set it, so a mistake
never waits until the next start to show up.

Put a setting back to its default with `longtable config unset <setting>`.

### Which database

Every command works on `longtable.db` in the folder you run it from. Point them
somewhere else with `-db`:

```bash
longtable -db table-two.db
```

That's how you run a second server on one machine — its own database, its own
rooms, its own settings. Give the same `-db` to every command you run for it,
or you'll be changing the other server.

### Keeping a copy of your settings

```bash
longtable config export -o my-settings.toml
```

That writes every setting to a file you can read, keep, or copy to another
machine. Load it back with:

```bash
longtable config import my-settings.toml
```

Longtable doesn't read that file when it starts — your settings live in the
database, and this is a copy of them.

### If you can't reach the server

Set `addr` to something unreachable and you can't get back in to fix it. Start
Longtable with `-addr` to override it for one run:

```bash
longtable -addr :8080
```

Then set it to something that works.

The banner across the top of every page isn't a setting — see **A message for
everyone on the server**, below.

### Players dropping in and out

When someone's connection drops, the room waits 30 seconds before showing them
as gone. Coming back inside that window changes nothing on anyone's screen, and
puts nothing in the chat log.

On a bad network, give it longer:

```bash
longtable config set departure_grace 2m
```

A player who closes their laptop still shows as connected until the wait is up.

### Upload limits

Every upload is checked against two limits: how big any one upload may be, and
how much this server will ever store in total.

The total counts each picture once, no matter how many rooms use it — two
rooms with the same map only cost the space for one copy. An upload that
would push the total past its limit is refused even if it's well under the
per-upload limit on its own.

```bash
longtable config set uploads.max_asset_size "50 MiB"
longtable config set uploads.max_total_storage "20 GiB"
```

## A message for everyone on the server

Put a line across the top of every page, in every room, until each person
dismisses it:

```bash
longtable set-banner "Maintenance 19 Aug 8-9pm EST"
```

Take it down again with:

```bash
longtable clear-banner
```

Both work while the server is running. Nothing needs restarting, and the
change reaches everyone the next time their page asks for it.

Run them from the folder holding your database, or add `-db
path/to/longtable.db`.

Change the message and it comes back for anyone who already dismissed the
last one.

## Getting a GM back into their room

Longtable was designed to be hosted and administrated by people who aren't
necessarily the GMs of every room. If someone loses GM access to a room because
they lost the room code or forgot the password, only the host can help recover
access.

Two recovery scenarios are documented here for hosts to help GMs that don't have
access to the hosting machine, and each should take less than a minute to do.
You will need to run the commands in a terminal!

Run them from the folder holding your database, or add `-db
path/to/longtable.db`. That's how these commands reach the same database the
server is using.

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
