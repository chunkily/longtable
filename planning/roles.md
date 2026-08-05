# Roles

Reference glossary for who's who. User stories should use these terms consistently.

- **Host** — Runs the server (self-hosts the Longtable instance). Not necessarily part of any room.
- **GM** — Runs a room's game. A room can have more than one GM. A subset of Room Members.
- **Player** — Everyone else in a room who isn't a GM. A subset of Room Members.
- **Room Member** — GM or Player; anyone participating in a room, regardless of role. Use this when a story applies to both.
- **Developer** — Builds and maintains Longtable itself. Responsible for the codebase, release process, and CI pipeline (expected to run via GitHub).

There is deliberately no role for "someone browsing the server who hasn't joined anything". One
existed briefly — Visitor — and it only had a job because of a public room list, which was itself
never a considered decision. The two propped each other up: the role's definition cited browsing
the list, and the list's audience was the role. Both are gone. You reach a room by being given its
link, so there is no state between "hasn't heard of this room" and "is in it" worth naming.

A Host is not a GM. They run the server and needn't be at any table on it, which is why the
recovery path for a GM who has lost a link or a password runs through them
([host-restores-room-access](user-stories/host-restores-room-access.md)) rather than through
anything in the web UI.

**A Player is a seat, not a browser.** One person on a phone and a laptop is one Player in one
chair, and someone who clears their browser data is the same Player when they sit back down —
see [ADR-0008](decisions/0008-seats-and-sessions.md). Roles here describe people at a table, and
the data model is only lately catching up to that.
