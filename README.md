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

## Running locally

```bash
cd web && npm install && npm run build && cd ..
go build -tags nodynamic -o longtable ./cmd/longtable
./longtable
```

Serves on `:8080` by default (`-addr` and `-db` flags to override).

## Running the tests

Three suites. All of these run from the repo root, and CI runs all of them on
every push and pull request.

**Go** — the sync hub, the store and its migrations, the REST API, the dice
parser:

```bash
go test -tags nodynamic ./internal/... ./cmd/...
```

Scope it to `./internal/... ./cmd/...` rather than `./...`: the repo root also
contains `web/node_modules`, which the go tool would otherwise walk looking for
packages. CI additionally runs with `-race`, which needs cgo and so won't work
against the CGO-free SQLite driver on a default Windows setup — Linux CI covers
it.

`-tags nodynamic` keeps the WebP encoder on its pure-Go path rather than picking
up a system libwebp if one happens to be installed, so a build behaves the same
on every machine. Leaving it off still compiles and passes, but you'd be testing
a different encoder from the one a Host's downloaded binary uses.

**Frontend unit tests** — the WebSocket client's state handling, and the pure
geometry modules (grid distance, drawing hit-testing). Vitest, no browser:

```bash
npm --prefix web run test
```

**End-to-end** — Playwright, driving real browsers against a real server. It
builds the Go binary and starts both the backend and the dev server itself, so
there's nothing to start first:

```bash
cd web && npx playwright test
```

The e2e suite needs ports **8080** and **5173** free, and writes to its own
scratch database under `web/.e2e-data/` (never your local `longtable.db`). First
run needs the browser: `npx playwright install chromium`. On Windows the first
run may also raise a firewall prompt for the freshly built server binary — it's
built to a fixed path, so approving it once is enough.

Several specs work by reading pixels off the Konva canvas, since the map has no
DOM to assert against. To run just one while working on it:

```bash
cd web && npx playwright test measure.spec.ts
```

Two more checks that aren't tests but will fail CI:

```bash
npm --prefix web run check && npm --prefix web run lint
```

`check` is svelte-check (types), `lint` is prettier plus eslint;
`npm --prefix web run format` fixes formatting.

## Documentation

See [`docs/`](docs/) for guides on hosting and configuring a server.
