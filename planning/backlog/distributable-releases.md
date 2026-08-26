---
title: Prebuilt distributable releases
created: 2026-07-29
status: open
tags: [host, release]
story: host-download-and-run-distributable
---

Publish prebuilt distributables so a Host can download and run the server directly, with no
build toolchain, compiler, or system libraries required on their machine.

**Everything this needs now exists** (2026-08-24), from
[automated-release-ci](automated-release-ci.md): `.github/workflows/release.yml` builds all six
targets and publishes them to a GitHub Release on a `v*` tag, and the README's Quickstart is
written against that. The binary itself is proven — a cross-built one was run on a machine with
nothing installed and served the whole app.

This stays `open` because nothing is *published* yet, and the step that publishes it is cutting
the first tag. That is a decision rather than work: it settles what version this project is at,
and it is the GM's to make. Push a `v*` tag and this closes, along with
[host-download-and-run-distributable](../user-stories/host-download-and-run-distributable.md),
whose first criterion is the only one still false.

**A second thing now wants this**: [check-for-new-versions](check-for-new-versions.md) cannot
tell whether a binary is behind without knowing what it is, so the stamp below is worth doing once
for both rather than twice.

Worth deciding at the same time: the binary has no way to say which version it is. The folder
inside the archive carries the tag, and nothing else does. A `-version` flag wants an `ldflags`
stamp in the workflow, which is a small change to make while cutting the first release and an
awkward one to want afterwards.
