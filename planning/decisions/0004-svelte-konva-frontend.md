# ADR-0004: SvelteKit + Konva for the frontend

**Status:** Accepted
**Date:** 2026-07-27 (documented retroactively on 2026-07-29 — see note below)
**Deciders:** Developer

> Like [ADR-0001](0001-self-hosted-multi-room.md) and [ADR-0002](0002-go-backend.md), this
> decision predates this repo's planning process and is documented retroactively — in this case
> from reasoning the Developer had already worked through in a separate session and supplied
> directly, which this ADR preserves close to verbatim.

## Context

Longtable's UI has two very different halves: a canvas-heavy map/token/fog/drawing surface, and a
surrounding shell of fiddlier, frequently-updating UI (chat, HP bars, connection status,
initiative tracker). The frontend framework and canvas library both needed to be chosen with that
split in mind, plus the self-hosted distribution model from [ADR-0001](0001-self-hosted-multi-room.md):
the frontend is downloaded/served fresh by hobbyist GMs rather than sitting behind a CDN with
long-lived caching, so shipped bundle size matters more than it would for a typical hosted web
app.

## Decision

Use SvelteKit for the frontend framework and Konva for the map/token canvas.

## Options Considered

### Framework — Option A: React

**Where this would actually have been the better call:** if the project expects to attract many
outside contributors, React's larger ecosystem and the "more contributors already know it" effect
is a genuine advantage for an open-source project trying to grow a community. This is a real
trade-off, not a strawman — Svelte was judged the better *technical* fit, React might be the
better *community* fit.

### Framework — Option B: Svelte(Kit) — CHOSEN

**Pros, in the Developer's own reasoning:**

- **Performance profile fits a canvas-heavy app.** Svelte compiles away most of its runtime
  overhead and doesn't use a virtual DOM — for a VTT, most of the actual rendering (map, tokens,
  fog) happens on a `<canvas>` anyway, entirely outside the framework's reactivity. Svelte's
  advantage shows up more in the surrounding UI — chat panel, initiative tracker, token property
  panels — where its reactivity model (`let x = 5; x = 6` just works) tends to produce less
  boilerplate than React's `useState`/`useEffect` dance, especially for the kind of fiddly,
  frequently-updating UI a VTT has (HP bars, live chat, connection status).
- **Bundle size matters more than usual here.** Since this is self-hosted and downloaded/served
  fresh by hobbyist GMs (not sitting behind a CDN with caching, per the context above), a smaller
  shipped bundle is a small but real win for time-to-first-playable-session.

### Canvas library — Option C: PixiJS

**Cons:** PixiJS's core strength is high-performance rendering of very large numbers of objects
(the kind of scale a particle system or a game with hundreds of moving sprites needs). Longtable's
scenes — a handful of tokens, drawings, and fog cells per map — never approach that scale, so
PixiJS's main advantage doesn't apply here, and its added complexity isn't justified by the
workload.

### Canvas library — Option D: Raw Canvas API / hand-rolled

**Cons:** Rejected due to high risk of incorrect behavior. Hand-rolling hit-testing, dragging, and
layering primitives from scratch raises the chance of subtle bugs in exactly the interactions
(moving tokens, drawing, revealing fog) that define whether the tool works at all — directly
against the core value proposition of providing a minimally functional, *correct* VTT.

### Canvas library — Option E: Konva — CHOSEN

**Pros:** A 2D scene graph purpose-built for draggable/interactive shapes (tokens, drawings, fog
cells), sized right for Longtable's actual object counts — without PixiJS's larger,
performance-oriented surface area that this workload doesn't need, and without the correctness
risk of implementing the same primitives by hand on the raw Canvas API.

## Trade-off Analysis

For the framework choice, the deciding factors were the performance fit (most rendering already
happens outside the framework, on canvas, so the framework's job is mainly the surrounding
fiddly UI where Svelte's reactivity model is more concise) and bundle size (a real cost given
self-hosted distribution, per [ADR-0001](0001-self-hosted-multi-room.md)). Both favor Svelte over
React on pure technical fit for this project's shape. The named counter-case — React's ecosystem
advantage for attracting outside contributors — was acknowledged directly as a real trade-off,
not dismissed, but wasn't the deciding factor here.

For the canvas library, the two rejected options failed on opposite ends: PixiJS's strength (huge
object counts) doesn't match a workload of a handful of tokens/drawings/fog cells per scene, while
hand-rolling on the raw Canvas API carries real risk of getting the core interactions (drag,
hit-test, layering) subtly wrong — a correctness risk the project's minimal-but-functional goal
couldn't absorb. Konva's purpose-built scene graph is sized right for the actual workload without
either downside.

## Consequences

- Most of the app's actual rendering (map, tokens, fog, drawings) lives in Konva/canvas code,
  largely outside Svelte's reactivity — component state changes need to be explicitly pushed into
  Konva's own object graph rather than relying on declarative re-rendering the way a DOM-based UI
  would.
- If the project later prioritizes attracting outside contributors over the current technical
  fit, this is the one part of the stack explicitly flagged as worth reconsidering — see the
  React trade-off above.

## Action Items

None — already implemented at initial commit. No further action from this ADR.
