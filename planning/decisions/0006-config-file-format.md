# ADR-0006: TOML for the host config file

**Status:** Accepted
**Date:** 2026-07-29
**Deciders:** Developer

## Context

Longtable needs a persisted server config file for Host settings (see
[host-config-file](../user-stories/host-config-file.md)): on startup, the server looks for it and
creates one with sensible defaults if it's missing, and it replaces environment-variable
configuration entirely (e.g. [host-configure-asset-limits](../user-stories/host-configure-asset-limits.md)).

The format was initially chosen as JSON via Go's standard library (`encoding/json`), on the
reasoning that it's the only common config format needing zero external dependencies, consistent
with [ADR-0005](0005-webp-reencoding-library.md)'s preference for avoiding dependencies where a
stdlib option covers the need.

That reasoning didn't hold up: this file is meant to be hand-edited by Hosts, and JSON has no
comment syntax, so a Host editing it has no way to see what a setting does, its valid range, or
why a value was chosen, without leaving the file to consult separate documentation. For a
settings file specifically meant to be understood and edited by a human, that's a real usability
problem, not just a stylistic preference — worth accepting a dependency to fix.

## Decision

Use TOML for the config file.

## Options Considered

### Option A: JSON (`encoding/json`, stdlib) — original choice, reversed

| Dimension | Assessment |
|-----------|------------|
| Complexity | Lowest — zero dependencies, already in the standard library |
| Cost | None |
| Human-editability | Poor — no comment syntax at all |
| Team familiarity | High — ubiquitous format |

**Pros:** No dependency, consistent with the project's general dependency-minimalism.
**Cons:** Cannot hold comments. A Host editing settings has no in-file way to know what a field
does, its default, or its valid values — directly at odds with the config file existing so Hosts
can configure the server without reading Go source.

### Option B: YAML

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — external dependency, and a larger/trickier spec to get right |
| Cost | One dependency (e.g. `gopkg.in/yaml.v3`) |
| Human-editability | Good on the surface, but has real footguns |
| Team familiarity | High — very widely recognized (Docker Compose, Kubernetes) |

**Pros:** Comments supported. Most broadly recognized format among self-hosters, so many Hosts
would already be comfortable with its syntax.
**Cons:** Indentation-sensitive (a misplaced space silently changes structure), and has
well-known implicit type-coercion surprises (e.g. an unquoted `no`/`yes`/`on`/`off` parsing as a
boolean rather than a string — the "Norway problem"). These are exactly the kind of mistake a
Host hand-editing a file without validation tooling is likely to hit.

### Option C: Simple `.env`-style (`KEY=value`, `#` comments)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Lowest among comment-supporting options — a hand-rolled parser is trivial (~20 lines), no library strictly required |
| Cost | Near-zero |
| Human-editability | Simple, but limited |
| Team familiarity | High — very simple format |

**Pros:** Comments trivial. Smallest possible implementation, effectively no dependency.
**Cons:** No nested/grouped structure — everything is flat `key=value`, which gets awkward once
settings need grouping (e.g. asset limits vs. room defaults). Also conceptually close to the
environment-variable configuration this file is explicitly replacing, undercutting the reason for
introducing a config file in the first place.

### Option D: TOML — CHOSEN

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — external dependency (e.g. `BurntSushi/toml` or `pelletier/go-toml`), but a small, well-scoped spec |
| Cost | One dependency |
| Human-editability | Good — comments supported, explicit types, no indentation sensitivity |
| Team familiarity | Medium — less universally known than YAML, but common in Go tooling specifically (e.g. Hugo) |

**Pros:** Comments supported. Simpler and more predictable than YAML — no indentation
sensitivity, no implicit type coercion — while still supporting grouped/nested settings unlike
the `.env`-style option. Well-precedented choice for exactly this use case in the Go ecosystem.
**Cons:** Requires an external dependency, same as YAML. Slightly less broadly recognized as a
format than YAML, though still common.

## Trade-off Analysis

Once comments are a hard requirement, Option A is eliminated outright — it cannot satisfy the
actual need. Between the two comment-capable structured formats, YAML's popularity is offset by
real hand-editing risk (indentation and type-coercion mistakes are easy to make and easy to miss,
especially for a Host without a YAML linter in their workflow) — a bad trade for a project that
otherwise wants distribution to non-experts to be simple. TOML gives up a small amount of
recognizability for a format that's harder to get subtly wrong by hand, which matters more here
than broad familiarity. The `.env`-style option was close on simplicity, but sacrifices grouped
structure and reintroduces the flavor of the environment-variable approach this feature is meant
to move away from.

## Consequences

- The config-reading code takes on one external dependency, where the original JSON plan had
  none — a reversal of the zero-dependency preference set in ADR-0005, justified here because the
  dependency buys a real usability requirement (comments) rather than convenience.
- The auto-generated default config file, and any documentation (see
  [host-config-documentation](../user-stories/host-config-documentation.md)), can use inline
  comments to explain each setting directly where a Host will see it, rather than requiring a
  trip to `docs/hosting.md` for basic context.
- TOML's stricter, simpler syntax should reduce hand-editing mistakes relative to YAML, at the
  cost of the format being somewhat less immediately recognizable to Hosts coming from a
  Docker/Kubernetes background.

## Action Items

1. [x] Add a TOML library dependency (e.g. `BurntSushi/toml` or `pelletier/go-toml`) for reading
   and writing the config file
2. [x] Ensure the auto-generated default config file includes explanatory comments for each
   setting, not just bare key/value pairs

Both done in `internal/config` (2026-08-18). The library is `pelletier/go-toml/v2`, chosen over
`BurntSushi/toml` for its strict decoding: it names an unrecognised key *and* draws the line it
sits on, which is most of the "malformed config fails with a clear error" criterion for free.

Writing turned out not to need the library at all. A marshaller emits key/value pairs and drops
comments, which is the one thing this ADR chose TOML for — so the generated file is a template
string with the defaults interpolated, and the library only ever reads. The consequence is worth
knowing: Longtable writes that file once and never rewrites it, because a rewrite would take a
Host's own comments away.
