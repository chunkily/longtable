# ADR-0009: An admin UI, and settings in the database

**Status:** Accepted
**Date:** 2026-08-25
**Deciders:** Developer

## Context

Longtable has no admin UI, and that was a decision rather than an omission. Three reasons held it
off:

1. A configuration UI served over HTTP is a large thing to expose, and doing it properly means
   building authentication for it.
2. The Host was assumed to be technical — someone who would open a TOML file and edit it.
3. It could be built later, once the parts a table actually uses had landed.

All three have moved. **The second assumption has been dropped**: the product is aimed at a Host
who downloads a binary and double-clicks it, and telling that person to hand-edit
`longtable.toml` is asking them to do the one thing they downloaded a binary to avoid. The third
is satisfied — rooms, scenes, tokens, fog, drawings, chat and initiative have all shipped. That
leaves the first, which is not a reason to decline but a specification for how to do it.

[ADR-0006](0006-config-file-format.md) chose TOML for the config file, and its reasoning was
explicit that the file "is meant to be hand-edited by Hosts" and exists "so Hosts can configure
the server without reading Go source". The format was never the problem. The premise was.

**Dropping the technical-Host assumption is not the same as replacing it with the opposite one.**
Both Hosts are real, and the requirement is to serve both rather than to trade one for the other:
installation and setup have to work for someone who has never opened a terminal, while someone who
wants to script a deployment, run headless, diff their configuration or automate a rebuild keeps
full control over every setting. A design that makes the easy path easy by making the expert path
impossible has failed, and so has one that keeps the expert path and calls a wizard "later".

Three constraints shape what replaces the file:

- **The database's own location cannot live in the database.** Something outside has to say where
  the file is.
- **A server has to be configurable without a browser.** A Host running headless over SSH is the
  case the UI cannot serve, and it is also — concretely — how the e2e harness works today, which
  writes a `longtable.toml` to set `departure_grace = "2s"` because the real half-minute outlives
  Playwright's timeouts.
- **Nothing may be reachable only through the UI.** The moment one setting is UI-only, headless
  Hosts have lost, and so has anyone automating a deployment.

## Decision

**Serve a token-protected admin UI, move settings into the database, and delete the config file.**

- Settings live in a `setting` table, edited from an admin UI and from a `longtable config` CLI.
- **The two are peers, and the CLI is the one that must be complete.** The UI is a front end over
  the same settings the CLI reads and writes, never a privileged path with something extra behind
  it. A setting that can only be reached by clicking is a bug.
- The config file is removed outright. There is no migration and no compatibility path: nothing is
  released yet, so there are no files in the world to migrate.
- Bootstrap — the two things that cannot come from the database — moves to flags: `-db` names the
  database, `-addr` overrides the stored listen address. Both have defaults, and a flag wins over
  a stored value so a Host cannot lock themselves out of the UI that fixes the setting.
- The admin UI is protected by a long random token in the manner of Jupyter: generated on first
  run, stored, printed to the console at startup as part of a URL, and exchangeable for a session.

## Options Considered

### Option A: Keep the TOML file, add a UI that edits it

| Dimension | Assessment |
|-----------|------------|
| Complexity | High — the file is written from a template precisely so comments survive |
| Cost | A comment-preserving TOML editor, which `go-toml` is not |
| Team familiarity | High |

**Pros:** A technical Host keeps the file they already know. No new storage.
**Cons:** ADR-0006 chose TOML *for the comments*, and the implementation writes the file from a
template and never rewrites it for exactly that reason. A UI that saves settings either destroys
the Host's comments on first save — throwing away the thing the format was chosen for — or needs
round-trip-preserving edits, which is a hard problem to take on for a settings screen. Two writers
of one file also invites the file and the UI disagreeing while the server is up.

### Option B: Settings in the database, config file kept as a stub

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — two places settings can live, forever |
| Cost | Low |
| Team familiarity | High |

**Pros:** Keeps a familiar text file for the bootstrap values, so recovery is a text editor.
**Cons:** A file that holds two keys is still a file to find, explain, document and support, and
it re-creates the confusion `host-config-file` was written to remove — a Host who edits the stub
expecting it to hold the settings it used to hold. Flags carry the same two values with nothing to
find.

### Option C: Settings in the database, no config file — CHOSEN

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — a settings table, a UI, a CLI, and token auth |
| Cost | One new HTTP surface to secure |
| Team familiarity | Medium — token auth is new here; the session mechanism is not |

**Pros:** One place settings live. A non-technical Host never meets a file. Values are validated
when they are entered rather than at the next startup, so a bad setting is refused at the point
someone can still see what they typed — today a typo stops the server booting. Settings become
changeable without a restart wherever the code is plumbed for it, which is what the banner already
does. Backing up the database backs up the configuration with it.
**Cons:** Nowhere to write the comments ADR-0006 bought — the UI and the CLI have to carry that
documentation instead, and if they carry it badly the Host is worse off than with a commented
file. Recovery of a bad value needs the binary rather than a text editor, which is why `-addr`
exists as an override. A new authenticated HTTP surface is a new thing to get wrong. **And a
technical Host loses the single artefact they could read, diff and check in**, which a sequence of
`config set` commands only partly replaces — see Consequences for what has to exist to make that
good rather than merely possible.

## Trade-off Analysis

The decisive argument is that Option A cannot be done well. The config file's own doc comment
records that it is written from a template and never rewritten, because a marshaller drops
comments and comments are the whole point. A settings UI is a writer. Adding one means either
abandoning that property or solving comment-preserving round-trips, and neither is a good trade
for a screen whose purpose is that the Host never has to read the file.

Between B and C, the question is whether a two-key file earns its existence. It does not: the two
values it would hold are exactly the two a flag carries without anything to locate, and keeping a
familiar filename that no longer holds the familiar settings is worse than removing it. Dropping
setting flags was the right call when a file held everything ([host-config-file](../backlog/host-config-file.md)),
and the reasoning — one way to name the database, so a Host cannot administer a file the running
server never opened — is preserved rather than reversed: `-db` becomes the single way again.

The security cost of C is real and is accepted with its limits stated rather than waved past. See
Consequences.

## Consequences

**The token is a real gate, and its limits are honest ones.** It is 32 bytes from `crypto/rand`
(`auth.NewToken`, which already exists and is already URL-safe), compared in constant time, and
printed at startup as part of a URL the way Jupyter does. What it protects against is anyone on the
table's network opening the admin page — which matters, because
[ADR-0007](0007-the-table-is-trusted.md) trusts the *table*, and server configuration is not a
table matter: a Host runs the server and need not be at any table (`roles.md`). What it does not
protect against is someone reading the LAN: this is plain HTTP with no TLS, so the token crosses
the wire in the clear, exactly as Jupyter's does without TLS. That is a limitation to write down
in `docs/hosting.md`, not one to pretend away.

**The token must be recoverable.** A console scrolls, and on Windows it closes with the process.
A Host who loses the line has to be able to get it back, and to invalidate it, from the CLI.

**A CLI has to exist regardless of the UI.** Headless Hosts need it, and so does the e2e harness,
which today writes a config file and would otherwise have no way to set `departure_grace`. This is
not an optional companion piece; the UI cannot ship without it.

**A technical Host has to end up with more control than the file gave them, not less.** The file
had one property worth keeping: it was a single artefact that could be read, diffed, version
controlled and copied to a second machine. `config set` typed one line at a time does not replace
that, so the CLI needs `config list` that prints every setting with its value and default, and an
export and import pair that round-trips the lot. Then a Host who wants their configuration in git
has it, a Host rebuilding a server replays one command, and the "wizard" and "text file" workflows
are two views of the same state rather than two systems. Without this the expert path is *possible*
and unpleasant, which is the failure mode this decision is supposed to avoid.

**The setup wizard is a first-run convenience, not a gate.** It runs when there is nothing
configured and a terminal is attached, and it must be skippable and non-interactive-safe: a
service, a container, a CI harness and a Host who passes flags all have to reach a running server
without answering anything. Defaults have to be good enough that answering nothing is a valid
choice.

**First run opens the Host's browser at the admin URL.** This is the moment the whole
non-technical path turns on: double-click, and the thing you configure it with is simply there,
rather than a console line you have to notice, understand and copy. Syncthing does exactly this
and is the nearest analogue there is — self-hosted, LAN, GUI on loopback. Four guards make it
safe rather than obnoxious, and the third is not optional:

- **It cannot delay or fail the start.** Fired after the listener is up, in a goroutine, with the
  error ignored. A machine with no default browser, or an `xdg-open` that hangs, must cost
  nothing — the console still prints the URL, which stays the real answer.
- **First run only**, keyed on an explicit `setup.completed` flag rather than on the database
  looking empty. A server that opened a browser on every restart is a server people learn to
  dread.
- **Suppressible by a flag**, because *the e2e harness wipes its database on every run and is
  therefore always a first run* — 174 specs would each launch a browser. Containers, services and
  anyone scripting a deployment need the same switch for the same reason.
- **Skipped where there is no desktop.** No `DISPLAY`/`WAYLAND_DISPLAY` on Linux means a NAS or a
  Pi, and a Windows service has no interactive session to open anything into.

**The token must not linger in the URL bar.** Opening a browser at
`…/admin?token=…` puts the token into history and into any referrer the page emits. The page
should exchange it for a session immediately and drop the query string, which is the same
correction Jupyter made after shipping the naive version.

**Startup output changes.** The server already prints the addresses a Host can hand out. It now
also prints an admin URL carrying a token, and that line is the most important thing on the screen
for a first-time Host.

**Some settings apply live and some cannot.** `addr` needs a restart by nature. `departure_grace`
is handed to the hub at construction and would need plumbing to change under a running room. The
UI has to say which is which rather than implying everything takes effect on save.

**`internal/config` mostly dissolves.** The defaults and the validation survive and become the
rules the settings table is written through; the file, the template, the parser and their tests
go.

**Uploaded art stays on disk, so `assets` stays a setting.** Moving blobs into the database was
considered as part of the same "what is an installation?" question and rejected. It would have
made the backup promise honest in one stroke — `docs/hosting.md` already says the database is
"the file to back up", which is untrue today because art lives beside it — and it would have
dissolved the two-step write that `blobstore` and `FindOrCreateAsset` carry scar tissue for. The
deciding fact is narrower than either: **`modernc.org/sqlite` exposes no incremental blob I/O**,
only whole-`[]byte` bind and column reads, so every upload and every download would materialise
the entire file in memory. At the 25 MiB default cap, a table switching to a new map means one
full copy per connected browser, where `io.Copy` from an `*os.File` is O(1) whatever the size.
Reclaiming space after a deletion would also need `VACUUM` rewriting the whole file, and a single
growing database defeats incremental backup in a way a content-addressed directory does not.
Serving large immutable bytes is what an operating system is good at. The consequence to keep
sight of is that "back up one file" remains false, and `docs/hosting.md` needs correcting to say
so rather than being left to imply otherwise.

## Action Items

- [ ] `setting` table, with the values Go validates on write — no `CHECK` constraints, per the
      schema rule that a growable value set is enforced in Go
- [ ] `-db` and `-addr` flags; delete `-config` and `internal/config`'s file handling
- [ ] Admin token: generate, store, print at startup, verify in constant time, regenerate from CLI
- [ ] `longtable config get`/`set`/`list`, plus `export`/`import` so a configuration is still one
      artefact a technical Host can diff and replay
- [ ] An admin-token command: print the current one, and replace it
- [ ] A first-run wizard, skippable and never blocking a non-interactive start
- [ ] Open the default browser at the admin URL on first run, behind the four guards above, and
      a flag to suppress it that the e2e harness passes
- [ ] `longtable admin` to open (or print) the admin URL on demand, which is also the answer to a
      Host who lost the startup line
- [ ] The admin UI itself, behind the token
- [ ] Repoint the e2e harness at the CLI instead of a written config file
- [ ] Rewrite `docs/hosting.md`'s settings section, including the plaintext-token caveat
