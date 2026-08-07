---
title: Run the e2e suite against the Go binary, not the dev server
created: 2026-08-07
status: done
tags: [testing]
---

The suite ran against `npm run dev`, with vite proxying `/api` and `/ws` to a Go binary the same
harness had already built. So every run tested a dev server no user runs, while the artifact a
Host actually starts — one binary serving the SPA it embeds — was built and then only used as an
API backend. CI even said so out loud: *"The specs themselves are served by vite, not by this
build."*

## What shipped

`e2e/run-backend.mjs` became `e2e/run-app.mjs`: it builds the frontend, builds the Go binary that
`go:embed`s it, and runs that. `playwright.config.ts` lost its second `webServer` entry and points
`baseURL` at `:8080`.

**This was a bug fix, not tidying.** Vite discovers dependencies lazily: the first page load that
reaches a new import triggers a re-optimize, and vite then tells every connected client to reload.

```
[vite] (client) optimized dependencies changed. reloading
TypeError: Failed to fetch dynamically imported module: …/nodes/3.js
```

A test whose page is reloaded mid-interaction loses what it was doing, and the symptom is a click
that lands and does nothing — `expect(page).toHaveURL(/\/r\/…/)` timing out on a page still
showing the form it had already filled in. On a cold `node_modules/.vite` the count of failures
matched the worker count **exactly**: 14 workers, 14 failures, each one a worker's *first* test.
That is what made it read as flakiness for so long — it only happens on the first run after
`npm install`, a config change or a cache wipe, and every run after it passes.

Verified by reproducing it deliberately (`rm -rf node_modules/.vite` before each run) and watching
14 failures become 0. The run also got faster: ~35s against ~50s, since nothing is being compiled
on demand any more.

Two things that came free with it:

- The SPA fallback in `internal/api/routes.go` is now exercised by every deep-linked spec, having
  previously been served by vite instead.
- `/api` and `/ws` are same-origin, as they are in production, rather than going through vite's
  dev proxy.

The cost is one `npm run build` (~6s) per run, and none at all in CI, which already builds the
frontend first because `go build` can't compile without `web/build`. The script always rebuilds
rather than checking freshness: six seconds is cheaper than an afternoon spent on a test failing
against code that isn't the code you just wrote.

For iterating on a failing spec with HMR, run `npm run dev` and a backend by hand and point
Playwright at `:5173` — just don't make that the default again.

## What this did not fix

Two rarer failures the vite problem was drowning out, together about one run in three:
[e2e-hang-after-token-edit](e2e-hang-after-token-edit.md). Different mechanisms — capping workers
makes the hang *more* frequent, which rules out the contention story that fits the vite one.

So the honest summary is that this fixed the loud, systematic cause and left two quiet ones. It is
still worth having done on its own terms: the suite now tests the shipped artifact, and a cold
first run stopped failing 14 tests.
