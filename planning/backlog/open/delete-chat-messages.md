---
title: Delete chat messages
created: 2026-07-29
tags: [chat]
---

GM can delete chat messages. Users can also delete their own messages.

There's no deletion capability at all today — no `DeleteMessage` in `internal/store/message.go`,
no `"chat.delete"`-style case in the WS handler switch (only `"chat.send"` exists,
`internal/ws/hub.go:207`), and no delete/soft-delete column on the `message` table. Needs a store
method, a WS command handler with authorization (GM can delete any message, a Player only their
own — via `Participant.Role`/`ID`, `internal/store/participant.go:14-27`), and a broadcast so the
deletion reflects in real time for everyone in the room.

## Related user stories

- [gm-delete-any-chat-message](../../user-stories/gm-delete-any-chat-message.md)
- [player-delete-own-chat-message](../../user-stories/player-delete-own-chat-message.md)
