# 26-10-15-260 — Review notes are written by the author and read by every reviewer

- **Status**: Accepted
- **Date**: 2026-10-15

## Context

The standard put a `REVIEW.md` beside a `DESIGN.md` as "a bullet list of
review rules and pitfalls", and said nothing more. It did not say when a note
is written, who writes it, or who reads it, and no repo had one. A discobox
also defines reviewers in `.discobox/review/*.md`, one subagent each. With
two kinds of Markdown file about review, it was not obvious whether a
`REVIEW.md` was a reviewer of its own.

The mistakes worth recording come up during review and during fixes: a
constraint the code cannot show, a mistake made a second time, a regression.
Unless they are written down where the next reviewer looks, each one is
forgotten once its branch merges.

## Decision

- A `REVIEW.md` is review notes for any agent, not a reviewer. Every
  reviewer reads the notes from the root down to each directory the change
  touches. The general reviewer is the main reader. A change that breaks a
  note is a finding.
- A note is added when something non-obvious turns up that a future review
  should check, when the same mistake is made a second time, or when a
  regression is fixed.
- The author writes the note, in the change that fixes what prompted it. A
  reviewer that thinks a note is due says so in its comment.
- A note goes in the closest directory that holds the code and has a
  `DESIGN.md`. `docs.review-design` stays.
- The rules are in the `repostd` skill, under Review notes, and in the
  starter `AGENTS.md`'s Package Design Docs section.

## Alternatives rejected

- **Review notes as reviewers in `.discobox/review/*.md`.** A reviewer
  definition is a separate subagent with a focus of its own. A pitfall in
  one package would cost a whole subagent on every review, including reviews
  that never touch that package. The notes would also sit away from the code
  they are about, and an agent changing the code, rather than reviewing it,
  would never read them.
- **Reviewers write the notes.** A reviewer edits nothing, so that the
  author cannot be reviewed by its own edits. A note is part of the change,
  so the author writes it, and the reviewer then sees it in the diff.
- **Dropping `docs.review-design` so a note can sit anywhere.** A note
  without a design beside it states a rule with no reason around it. The
  root always has a `DESIGN.md`, so a note always has a valid place.

## Consequences

- The general reviewer's brief, in discobox's `discobox-review` skill, has
  to name `REVIEW.md` and its order of reading explicitly. Today it says only
  "design or review notes".
- Notes pile up, so the skill says when to delete one, when to fix one, and
  when to move notes down a directory. Nothing checks a `REVIEW.md`'s length.

## Deferred

- A length rule like `docs.design-length` for `REVIEW.md`. Revisit when a
  repo's `REVIEW.md` has grown past what is read in one sitting.
