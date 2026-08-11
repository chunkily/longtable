# Longtable

A simple virtual tabletop (VTT) for playing Dungeons and Dragons online — for
hobbyist groups who want a way to run their game digitally without paying for a
subscription or software licenses using hardware they already own.

Longtable runs as a single program that one person in the group (usually the GM)
downloads and starts up on their own computer; everyone else just opens it in
their web browser to join.

## v1 scope

Core tabletop only: map upload, token placement/movement, fog of war, a basic
dice roller, and real-time sync between the GM and players. No character sheets
or rules automation yet.

## Building and running locally

```bash
cd web && npm install && npm run build && cd ..
go build -tags nodynamic -o longtable ./cmd/longtable
./longtable
```

Serves on `:8080` by default (`-addr` and `-db` flags to override). On startup it prints
the addresses everyone else can connect to — one per network interface, so you don't have to
go looking for your own IP:

```
INFO longtable: listening addr=:8080 db=longtable.db assets=longtable-assets
INFO longtable: reachable at url=http://192.168.1.23:8080 interface=Ethernet
```

`-banner "Back up at 9pm"` puts a message across the top of every page for
everyone on the server, which each of them can dismiss. Changing the text brings
it back for people who dismissed the last one.

## Running the tests

Three suites, all run from the repo root.

**Go:**

```bash
go test -tags nodynamic ./internal/... ./cmd/...
```

**Frontend unit tests:**

```bash
npm --prefix web run test
```

**End-to-end.** Install the browser once:

```bash
cd web && npx playwright install chromium
```

Then run the suite:

```bash
cd web && npx playwright test
```

## Documentation

See [`docs/`](docs/) for guides on hosting and configuring a server.

**Lost a room code, or a GM password?** Ask whoever runs the server. They can find a room by its
name or the date it was made, and reset a GM password without the old one:
[recovering a room](docs/hosting.md#getting-a-gm-back-into-their-room). There's no self-service
recovery, because rooms aren't listed anywhere public.
