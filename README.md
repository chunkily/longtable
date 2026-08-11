# Longtable

A **simple** virtual tabletop (VTT) for playing Dungeons and Dragons online — for
hobbyist groups who want a way to run their game digitally without paying for a
subscription or software licenses using hardware they already own.

Longtable runs as a single program that one person in the group (usually the GM)
downloads and starts up on their own computer; everyone else just opens it in
their web browser to join.

## Features

- Self hosted, so you own all the data.
- No accounts, no external logins.
- Rooms with real time sharing.
- Unlimited map scenes using your own art.
- Per token tracking of conditions and stats.
- Fog of war, painted per tile.
- Freehand and shape drawing, pings, distance and area-of-effect measuring.
- An initiative tracker.
- Chat, with a builtin dice roller.
- Light and dark themes.
- Mobile responsive UI.
- Fully open source with a permissive license.

Explicitly out of scope:

- Animated maps: an uploaded GIF becomes a still image
- Automated fog: All fog is hidden and revealed manually by a GM.

If these features are important to you, you might want to look elsewhere!

## Quickstart

TODO: Populate this section in the future when releases are ready.

## Documentation

See [`docs/`](docs/) for guides on hosting and configuring a server.

**Lost a room code, or a GM password?** Ask whoever runs the server. They can find a room by its
name or the date it was made, and reset a GM password without the old one:
[recovering a room](docs/hosting.md#getting-a-gm-back-into-their-room). There's no self-service
recovery, because rooms aren't listed anywhere public.

## Building

Prerequisites to build the application from source:

- [Go](https://go.dev) 1.26.5 or newer
- [Node.js](https://nodejs.org) 22 or newer, with npm

Then run the following.

```bash
cd web && npm install && npm run build && cd ..
go build -tags nodynamic -o longtable ./cmd/longtable
```

### Testing

This project has 3 test suites.

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
