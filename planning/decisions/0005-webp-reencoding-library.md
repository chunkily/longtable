# ADR-0005: WebP re-encoding via WASM-embedded libwebp

**Status:** Accepted
**Date:** 2026-07-29
**Deciders:** Developer

## Context

Longtable is a self-hosted Go VTT server. To protect Room Members from malicious content hidden in uploaded images (see [room-member-safe-asset-content](../user-stories/room-member-safe-asset-content.md)), every uploaded asset must be decoded and re-encoded server-side before being stored or served, and only PNG, JPEG, WebP, and GIF are accepted as input. WebP support specifically matters because it's commonly used for VTT background/map images, which tend to be large — lossy compression quality is not optional there.

Two other constraints, both already committed to, shape this decision:

- [host-download-and-run-distributable](../user-stories/host-download-and-run-distributable.md): Hosts must be able to download a single prebuilt distributable and run it directly, with no build toolchain, compiler, or system libraries installed on their machine.
- [developer-automated-release-ci](../user-stories/developer-automated-release-ci.md): Developer wants releases built and published automatically via a single GitHub Actions workflow that cross-compiles distributables for Linux/macOS/Windows on amd64/arm64, triggered by a version tag.

Go's toolchain can normally cross-compile for any OS/architecture from a single build machine just by setting `GOOS`/`GOARCH`, because pure Go code never calls out to the host's C compiler. That convenience disappears the moment a dependency requires cgo (a C bridge): Go automatically disables cgo when cross-compiling unless a matching C cross-compiler for the target platform is explicitly supplied, which a real WebP encoder (libwebp) is written in C and would require.

## Decision

Use a WASM-embedded build of libwebp — the `gen2brain/webp` package, which ships real libwebp compiled to WebAssembly and executes it through `wazero`, a pure-Go WASM runtime — for WebP decode and encode during asset re-encoding.

## Options Considered

### Option A: cgo bindings to libwebp (`chai2010/webp`)

| Dimension | Assessment |
|-----------|------------|
| Complexity | High — requires a C toolchain and libwebp at build time; breaks single-job cross-compilation |
| Cost | Higher CI cost — GitHub bills macOS runners ~10x and Windows ~2x Linux minutes |
| Scalability | N/A (build-time concern, not runtime) |
| Team familiarity | Low — Developer is not deeply experienced with GitHub Actions; more moving parts to debug |

**Pros:** Best encode quality/performance; matches the reference libwebp implementation exactly (native code).
**Cons:** Go disables cgo on cross-compile without a matching C cross-compiler, so the single-job release build stops working. The usual workaround — a per-OS build matrix with native Windows/macOS/Linux runners, each needing libwebp installed correctly — adds CI complexity, cost, and more independent failure points. Native arm64 coverage is also inconsistent across GitHub-hosted runners (e.g. no standard Windows-on-arm64 runner), which could leave a gap in the platform matrix the distributable story requires.

### Option B: Pure-Go native WebP encoder (`nativewebp`)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — no cgo, no WASM runtime, fits the existing single-job build unchanged |
| Cost | No added CI cost |
| Scalability | N/A |
| Team familiarity | High — plain Go dependency, nothing exotic |

**Pros:** Simplest option from a tooling standpoint; no cgo, no embedded WASM blob, no cross-compile impact at all.
**Cons:** Lossless-only today, producing much larger files than a properly lossy-compressed WebP. A poor fit specifically for large background/map images, which is the primary reason WebP support was requested in the first place.

### Option C: WASM-embedded libwebp (`gen2brain/webp` + `wazero`) — CHOSEN

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low from the build's perspective — no cgo anywhere in the toolchain, single-job cross-compile keeps working unchanged |
| Cost | No added CI cost; runtime cost is a slower/heavier re-encode per upload |
| Scalability | Re-encoding happens once per asset upload, not on a hot/request path, so per-call overhead is not a scaling concern |
| Team familiarity | Medium — dependency itself is unfamiliar, but requires no new build tooling or CI concepts |

**Pros:** Full libwebp encode/decode fidelity (real lossy compression, same as Option A) without a C toolchain anywhere in the build or release process. The existing single-job, loop-over-`GOOS`/`GOARCH` release workflow keeps working exactly as planned; Hosts still get a plain self-contained binary.
**Cons:** Larger binary (the compiled libwebp WASM blob is embedded in the distributable). Slower and more memory-heavy per-image re-encode than native code, since WASM is interpreted/compiled at runtime rather than running natively.

## Trade-off Analysis

The deciding factor is where each option's cost lands. Option A trades better runtime encode performance for a permanently more complex, slower, and costlier release pipeline (multiple OS-native runners, more failure points, inconsistent arm64 coverage) — a bad trade given the Developer explicitly wants a simple, fully automated single-pipeline release process and isn't yet deeply familiar with GitHub Actions. Option B keeps the build maximally simple but fails the actual requirement: background/map images need real lossy compression, not a lossless-only encoder that produces bloated files. Option C pays its cost at a different, cheaper point — a larger binary and a slower one-time re-encode per upload, which is invisible to Room Members since it never sits on a hot path (it only runs once, at upload time) — while preserving both the encode quality of Option A and the build simplicity of Option B.

## Consequences

- The release workflow can remain a single job that cross-compiles all platform/architecture targets, with no per-OS build matrix and no C toolchain setup required in CI.
- Hosts continue to get a single self-contained distributable with no runtime dependencies to install.
- The distributable binary will be larger than a pure-Go-only build due to the embedded WebP WASM module.
- Asset upload/re-encoding latency will be higher than a native cgo implementation; acceptable since it's a one-time cost per upload, not a per-request cost.
- If encode performance or binary size later becomes a real problem (e.g. very high upload volume), this decision should be revisited — at that point Option A becomes more attractive since the release pipeline would already need to mature for other reasons.

## Action Items

1. [x] Add `gen2brain/webp` as the WebP decode/encode dependency for the asset re-encoding pipeline
2. [x] Confirm the embedded WASM module is included correctly in cross-compiled builds for all target platforms — moot, see the update below: there is no longer a WASM module to include
3. [ ] Measure actual binary size and per-upload re-encode latency once implemented, to validate the trade-offs assumed here

## Update, 2026-07-30 — the mechanism changed, the decision holds

Implemented at `gen2brain/webp` v0.6.4, which no longer works the way this decision describes.
It doesn't ship a WASM module executed through `wazero`; it ships libwebp **transpiled to pure Go**
(generated by `wasm2go`), and `wazero` is gone from the dependency tree entirely. Every version
back to v0.4 available today behaves this way, so the wazero-based package this decision was
written against is simply not what's on offer any more.

The choice stands, and on better terms than assumed: still no cgo, still a single-job
cross-compile, but without a WASM runtime interpreting anything at upload time and without an
embedded WASM blob inflating the binary. The "larger binary, slower encode" consequences listed
above are therefore softer than predicted — action item 3 is still worth doing to say by how much.

One wrinkle the decision couldn't have anticipated: the package tries a **system libwebp** via
`purego` first and only falls back to the transpiled Go. That would make behaviour depend on
whether the Host happens to have libwebp installed, which is at odds with
[host-download-and-run-distributable](../user-stories/host-download-and-run-distributable.md)
promising one self-contained binary that behaves the same everywhere. Builds and tests therefore
pass `-tags nodynamic`, which takes the transpiled path always. That's the only reason the tag
appears in the build commands.

Encode quality was verified against Chrome decoding the same files with real libwebp: flat
colours round-trip within ±1 at q90. The decode side needs a correction, which is documented in
`internal/imageproc` and in
[safe-asset-reencoding](../backlog/done/safe-asset-reencoding.md) rather than here, since it's an
implementation detail of this library rather than a reason to have chosen differently.
