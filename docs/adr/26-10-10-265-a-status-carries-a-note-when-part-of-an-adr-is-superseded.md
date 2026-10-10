# 26-10-10-265 — A status carries a note when part of an ADR is superseded

- **Status**: Accepted
- **Date**: 2026-10-10

## Context

A status was exactly `Proposed`, `Accepted`, `Rejected` or `Superseded by
<ID>`, on one line. A later ADR often replaces one section of an earlier one
and leaves the rest standing. discobox records that on the earlier ADR as
`Accepted (§4 superseded by <ID>)`, about a hundred times, and such a status
is usually longer than a line. repocheck rejected the status, read only its
first line, and then reported the `Date` below it as missing. The generated
index showed the first line only, so discobox kept its index by hand.

## Decision

- A status is one of the four states, optionally followed by a note in
  parentheses. The note is free text; it names the part and the ADR that
  superseded, refined or narrowed it.
- A header value may wrap onto indented lines. The parser joins them.
- The generated index prints the whole status, note included.
- `Superseded by` and `Supersedes` still mean a whole ADR, and
  `adr.supersede` still pairs them. A note is not checked against the other
  ADR.

## Alternatives rejected

- **A `Superseded in part by` header field.** It would need its own list
  syntax for "which part, by which ADR", and the index would need a second
  column or would drop it. The note already reads as a sentence.
- **Pairing a note with the later ADR's `Supersedes`.** The later ADR would
  have to say which part it replaces in a form repocheck can match, and the
  hundred existing notes are prose.
- **Keeping the status to one line.** Links to two ADRs do not fit in one
  line, and the repos' Markdown wraps at 80 columns.

## Consequences

- discobox's index can be generated without losing its notes.
- Nothing checks that a note is true, so a note on the earlier ADR and the
  later ADR's text can disagree.
- An indented line directly below a header field is part of that field.
