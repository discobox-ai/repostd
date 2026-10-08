# 26-10-08-974 — ADR IDs are a date and a random suffix

- **Status**: Accepted
- **Date**: 2026-10-08

## Context

ADR 0001 made ADR filenames `NNNN-slug.md`, numbered in sequence. A sequence
is allocated by whoever writes next, so two branches in flight both take the
next free number. discobox hit this four times, renumbered the duplicates,
and then moved to `YY-MM-DD-RRR` IDs (its ADR 26-09-24-458). Because of
that scheme, repocheck reported every discobox ADR written since then as
malformed, so discobox could not conform without renumbering ADRs it
treats as immutable.

## Decision

- An ADR's ID is `YY-MM-DD-RRR`: the date it is written plus three random
  decimal digits. It is generated, never the next number in a sequence.
- ADRs that already have four-digit `NNNN` IDs keep them. Both forms are
  valid everywhere an ID appears: the filename, the title, `Superseded by`
  and `Supersedes`.
- `Superseded by` and `Supersedes` accept an ID or a Markdown link whose text
  is the ID, `[<ID>](<file>)`.
- The generated index sorts by ID as a string, which puts the `NNNN` ADRs
  first and then the dated ones in date order.
- If two branches draw the same ID, the unmerged one re-rolls its suffix.
  `adr.unique` catches the collision when the second one merges.

## Alternatives rejected

- **Keep `NNNN` and renumber on collision.** Renumbering breaks every
  citation of the old number, in code comments and in other ADRs, and a
  branch only finds the collision when it rebases.
- **A full timestamp or a longer random suffix.** These collide even less,
  but make the IDs that prose cites longer. Three digits under a date only
  collide when two ADRs are written on the same day, and then the
  collision is caught and cheap to fix.
- **Waive `adr.format` for repos that use dates.** Every repo with parallel
  branches has this problem, so a per-repo waiver would leave the standard
  wrong while it still checked none of those repos' ADRs.

## Consequences

- discobox's ADRs written since its own switch to dated IDs pass
  `adr.format`, so a waiver is needed only for headers that really are
  non-standard.
- Existing `NNNN` ADRs, including 0001 here, are valid unchanged.
- `repocheck` messages and the template say `<ID>` or `YY-MM-DD-RRR`
  instead of `NNNN`.
