---
title: Rooms and assets pages on the admin screen
created: 2026-10-29
status: open
tags: [host, admin, assets, moderation]
story: host-moderate-assets
---

The admin page is one screen of settings. Two more things a Host looks after belong beside it:

- **Rooms** — every room on the server, today only visible through `longtable room list`: name,
  room code, when it was made, when it was last used, how many seats, how much art. Reset a GM
  password from here as well as from the CLI.
- **Assets** — every stored image, across all rooms, with a thumbnail, which rooms have it in
  their library, its size, when it arrived, and the other assets made from the same upload
  (`asset_source` groups a portrait's crops and a map's alignments). Removing one is the
  moderation the [host-moderate-assets](../user-stories/host-moderate-assets.md) story asks for.

## Routing

No SvelteKit routing, and no client-side router. The admin page is server-rendered Go templates on
purpose (CLAUDE.md's `internal/admin` row says why: its bundle would otherwise ship to every
Player, and a form posting back runs the same `Parse` the CLI does), and more pages don't change
that reasoning. Go's `ServeMux` already does the routing these need:

```
GET  /{$}                  settings (today's page; "/" stops being a catch-all)
GET  /rooms                room list
GET  /assets               asset list, ?q= to search, ?room= to filter
POST /assets/{id}/remove   remove one, with an optional reason
```

`guard` wraps the whole mux, so every new page gets the token, Host-header and cookie checks for
free; a test should prove a new route can't be reached without them. A shared template layout with
a three-link nav (Settings, Rooms, Assets) is the only new structure.

**Thumbnails come from the admin listener, not the game one.** The two are different origins, and
the admin page shouldn't depend on the game address being reachable from wherever the Host is —
`admin_addr` and `addr` can be bound differently. Serve the blob from `internal/blobstore` through
an admin route instead.

## What removing does

That is the moderation story's acceptance criteria, and most of it is real work rather than a
page: delete the blob and the row, repoint anything using it to a placeholder, record the source
hash so a re-upload is refused ([track-removed-asset-hashes](track-removed-asset-hashes.md)), and
tell the uploader why. Worth building the list first and removal second, as two commits — the list
is useful on its own, and removal is where the irreversible part lives.

The story's criterion "accomplished via a CLI command or direct database/filesystem access, not a
web admin UI" predates the admin surface ([ADR-0009](../decisions/0009-admin-ui-and-database-config.md))
and is what this item contradicts. Rewrite it when this ships — the admin page is now the Host's
screen, and the reason for avoiding a web UI (building one at all) no longer applies.

- [ ] Shared layout and nav; settings moves to `GET /{$}`
- [ ] Rooms page, read-only first
- [ ] Assets page with thumbnails served by the admin listener, grouped by source upload
- [ ] Remove an asset, per the moderation story
