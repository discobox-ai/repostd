# 0001 — One standard for discobox-ai repos, enforced by repocheck and a skill

- **Status**: Accepted
- **Date**: 2026-09-22

## Context

discobox, bouncer, and obot-platform/obot grew their own conventions for
layout, docs, tooling, lint, CI, and release. Where they differ, each repo
also contradicts itself: discobox has two ADR header styles, duplicate ADR
numbers, 2,500-line "short" design docs, and a task that sets an env var no
test reads; obot has a Makefile advertising ghost targets, lint skipping two
modules, and flipped AGENTS.md/CLAUDE.md symlinks. A written standard alone
would drift the same way. This standard is for `discobox-ai` repos only.

## Decision

- The standard is one document, the `repostd` skill, distributed into every
  repo by `go tool repocheck sync`.
- Every mechanically checkable rule has a rule ID and a check in
  `repocheck`, run by `task check` in CI. The rest are judgment rules the
  skill applies. Deviations are waivers with reasons, in `.repocheck.yaml`.
- The choices (details in the skill):
  - root holds only `cmd`, `pkg`, `internal`, `docs`, `test`, `scripts`;
  - `AGENTS.md` is the real file, `CLAUDE.md` a symlink;
  - `DESIGN.md`/`REVIEW.md` sit next to the code, with a length budget;
  - `docs/adr/NNNN-slug.md` with a fixed template and a generated index;
  - Taskfile run by `go tool task`, with tools pinned in `go.mod`;
  - Nix provides every non-Go dependency CI uses, and CI installs nothing
    else;
  - one canonical golangci-lint config;
  - standard `testing` only;
  - thin workflows on Depot runners, with actions pinned by commit hash;
  - Renovate from a shared preset;
  - releases through Taskfile targets backed by Go programs.

## Alternatives rejected

- **A written standard without a checker.** Every repo surveyed documents
  rules that it then breaks; nothing but CI holds a rule in place.
- **A checker without the skill.** Half the standard (is this ADR-worthy,
  does this `DESIGN.md` describe current state, does this belong in `pkg/`)
  is judgment that a check cannot make.
- **Packages at the repo root (discobox).** It mixes stable contracts with
  top-level containers and makes "what is public" a per-directory question.
  `pkg/` versus `internal/` answers it structurally.
- **Makefile (obot).** Make offers no `go tool` pinning and poor Windows
  support, and obot's Makefile shows the targets rotting. A Taskfile is run
  by a `go.mod`-pinned binary on every OS.
- **goreleaser (obot).** Past a simple binary release we end up fighting its
  model. Release logic is Go programs behind `task release:*`, shared
  through repostd when generic.
- **Integration tests behind build tags (obot).** Tagged files do not
  compile in the default build, so they rot unnoticed. Env-var-gated
  `t.Skip` keeps them compiling, and `tests.env-read` catches a task that
  sets a variable no test reads.
- **testify (obot).** A second assertion dialect for no capability the
  standard `testing` package plus go-cmp lacks.
- **Actions pinned by version tag (both).** Tags are mutable. Commit hashes
  with a `# vX.Y.Z` comment are immutable, and Renovate keeps them current.
- **Nix optional, with `actions/setup-*` in CI.** CI and a developer would
  then resolve tools differently. The flake is the one environment; Windows,
  where Nix does not run, may use `actions/setup-go` and nothing else.
- **Per-repo golangci-lint configs.** golangci-lint cannot inherit a config,
  so per-repo copies drift. The config is managed, with named local blocks
  for real repo additions.
- **Date-named ADRs (obot) and a hand-kept index (discobox).** Numbers give
  a stable short reference. The index is generated because the hand-kept one
  drifted.
- **`CLAUDE.md` as the real file (obot).** `AGENTS.md` is the
  vendor-neutral name that other agents read.

## Consequences

- A standard change lands in repostd, and repos pick it up by bumping the
  tool and running `repocheck sync`; `task verify` fails until they do.
- discobox needs waivers for its root-level packages and `go.work` until it
  is restructured, and its ADRs need renumbering and header normalization.
- Every repo carries a flake, even a pure-Go one.

## Deferred

- **Full Mermaid parsing.** `docs.mermaid` checks only for a known diagram
  type, because the real parser is JavaScript. Revisit when a Go parser
  exists, or when every repo's flake already ships node.
- **Migrating discobox.** Its layout change is its own project. Until then
  it runs with waivers.
