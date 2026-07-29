---
title: Reconnect after the WebSocket drops
created: 2026-07-29
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
