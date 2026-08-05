---
title: Reconnect after the WebSocket drops
created: 2026-07-29
status: done
tags: [websocket, reliability]
---

A dropped connection currently ends the session until the page is reloaded.
`socket.onclose` (`web/src/lib/room.svelte.ts:179`) sets `status = 'closed'` and does nothing
else — there is no retry, no backoff, and nothing that tells the user the room is no longer live
beyond the small status badge in the header. Any wifi handover, laptop sleep, tunnel restart, or
`longtable` restart during a session lands here.

What makes it worse than a dead connection normally would be:

- Every command silently no-ops. `send` (line 199) returns false when the socket isn't open, so
  chat messages, token moves, and fog reveals just don't happen, with no error and no queue.
- Optimistic drawings and erases freeze in whatever state they were in. They're reconciled by the
  `state.sync` that a reconnect would deliver, so with no reconnect the map can keep showing a
  stroke the server never accepted, or keep hiding one it refused to erase. (This isn't caused by
  optimistic rendering — the connection has always been terminal — but it turns a stalled session
  into a visibly wrong one.)

## What's needed

- [ ] Retry on close with capped exponential backoff and jitter, reusing the saved session token
      (`loadSession`, `web/src/lib/session.ts:11`) — the participant already exists, so this
      re-opens the socket rather than re-joining the room.
- [ ] Surface the attempt in the UI. `ConnectionStatus` already has `'connecting'`, so a retry can
      reuse it; decide what to show when retries are exhausted (a banner with a manual "reconnect"
      is probably better than retrying forever).
- [ ] Stop retrying when a REST probe says the session is the problem rather than the server.
      `Hub.ServeHTTP` rejects an unknown room with 404 and an invalid session with 401, but a
      failed WebSocket handshake reaches the browser as a bare `onclose` with no status, so the
      socket alone can't separate "the server is restarting, keep trying" from "this session is
      gone, send them back to the join form". Probing over REST between retries is the agreed
      escape hatch.

      Which endpoint is still open. `GET /api/healthz` (`internal/api/routes.go:27`) only reports
      that the process is up — nothing in the REST surface validates a session token today, so
      healthz alone can only support a heuristic ("server answers but the socket won't open, so
      assume the session died"), which would bounce someone to the join form for a transient
      failure during startup. A small authenticated endpoint — a `GET` that runs the same
      `GetParticipantByToken` lookup the hub does and answers 200/401/404 — makes the distinction
      exact and is worth the few lines. Otherwise, pair healthz with a retry count so the bounce
      only happens after several attempts.

The reducer side needs no work: `state.sync` already replaces room, scene, tokens, fog, drawings,
and messages wholesale, and calls `resetPending()` (line 314) to drop anything that was in flight,
so a reconnect converges on server state by construction.

## Testing

Killing the backend mid-session is the quickest manual check. For an automated one, the
`routeWebSocket` interception in `web/e2e/drawing-optimistic.spec.ts` can close a live socket from
the test rather than only dropping frames.

## What shipped

All three checkboxes. Retry with capped exponential backoff and jitter (500ms doubling to a 15s
ceiling, eight attempts), a banner that says what is happening, and the authenticated probe this
item argued was worth the few lines — `GET /api/rooms/{slug}/session`, answering 200/401/404, the
same three answers the upgrade gives. That was the right call: the heuristic alternative
("healthz answers but the socket won't open") would bounce someone to the join form during a
server restart, which is the failure that loses work.

Points worth keeping:

- **It re-opens the socket, never re-joins.** The participant already exists; joining again would
  mint a second one, with a second name in the roster and a second presence badge.
- **`onclose` distinguishes a close we asked for from one that happened to us** by comparing the
  socket against `this.socket`, which `disconnect()` clears *before* closing. Without that,
  leaving the page starts a reconnect to a room nobody is in any more.
- **The probe runs before spending an attempt, not after failures pile up.** A dead session is
  the failure that retrying can never fix, and the one where a spinner explains nothing.
- **`unreachable` is a third answer, distinct from `invalid`.** A probe that can't get a reply
  means the server is down, which is the case retrying is *for* — treating a fetch failure as an
  expired session would bounce everyone out on the first blip.
- The first retry waits the base delay, not double it: `retryDelay()` is computed *before*
  `attempt` is incremented. Getting that backwards is invisible in use and was caught by a test
  advancing exactly 1000ms.

The reducer side needed nothing, exactly as this item predicted — `state.sync` replaces the whole
picture and `resetAfterSync` drops in-flight state, so a reconnect converges by construction.
There's an e2e that proves it rather than asserting it: chat sent while the socket is down never
arrives, and the log that comes back is the server's.

Not done: nothing queues commands sent while the connection is down. They still silently no-op —
the banner now says so, which was the cheap half of that problem. A real outbox would need to
decide what is still meaningful to replay after a `state.sync` has rewritten everything, and is
its own item if anyone wants it.
