# Repository Guidelines

## Project Structure

- Root module `github.com/discobox-ai/repostd`: the discobox-ai repository
  standard and `repocheck`, the tool that enforces it.
- The root holds only top-level containers (`cmd`, `pkg`, `internal`, `docs`,
  `test`, `scripts`), never packages.
- `cmd/repocheck`: the `repocheck` binary (check, sync, init, rules).
- `internal/canon`: the embedded files the standard distributes. The
  standard itself is `files/managed/.agents/skills/repostd/SKILL.md.tmpl`.
  Every embedded file has a `.tmpl` suffix.
- `internal/rules`: one `Rule` per rule ID the skill cites.
- `internal/managed`: expected managed content, and `Sync`.
- `internal/adr`: ADR parsing and the generated index.
- `internal/repo`: the repository view rules check, and waivers.
- `default.json`: the shared Renovate preset (`github>discobox-ai/repostd`).
- `docs`: ADRs (`docs/adr`).

Changing the standard means changing the skill text, the rule, and its test
together. This repo conforms to itself: after editing `internal/canon`, run
`go tool task sync` to refresh the root copies.

## Shared Libraries

Reuse `github.com/discobox-ai/x` packages when applicable instead of writing
local equivalents (`go get github.com/discobox-ai/x@main`). If a generic
helper would be useful beyond repostd, it belongs in `x`, not here.

## Git Workflow

Work directly on whatever branch is already checked out. Do not create new
branches or worktrees unless explicitly told to. Commit messages follow
Conventional Commits.

## Commands

Use Taskfile targets through the Go tool-managed `task` binary:

```bash
go tool task --list
go tool task build      # compile into build/
go tool task test       # run tests (-race)
go tool task check      # golangci-lint, repocheck, shellcheck, actionlint
go tool task generate   # regenerate generated files
go tool task verify     # fmt, go.mod, generated and managed files are current
go tool task ci         # everything CI runs: verify, check, test
```

At the end of a code-changing task, run `go tool task ci`. Add Taskfile
targets instead of documenting ad hoc commands here.

## Implementation Quality

Prefer proper structural changes over compatibility shims or narrow patches.

- Do not introduce optional interfaces for behavior the system requires. Add
  required methods to the core interface and update all implementations.
- Do not add wrapper types, adapter layers, or helper wrappers just to avoid
  touching callers. Change the call sites.
- Treat existing databases and persisted state as durable. Design schema
  changes with a safe upgrade path.
- When the correct fix crosses package boundaries, update the model,
  interfaces, implementations, tests, and call sites together.
- Keep abstractions justified by durable ownership or meaningful complexity
  reduction.

## Package Design Docs

`DESIGN.md` explains the design of a package and its subdirectories;
`REVIEW.md` lists its review rules and pitfalls. Read them from the root down
to the package you are changing; closer files override parents. Keep them
short and directive, describe current state only, and update them in the same
change as the code. See the `repostd` skill for the rules.

## Architecture Decision Records

`docs/adr` records decisions and the alternatives rejected. Write an ADR only
when a plausible alternative was rejected for a non-obvious reason, or
something was deferred with a condition for revisiting it. Land it as
`Proposed`; `Accepted` is the go-ahead. ADRs are immutable once shipped
against: supersede, never edit. See `docs/adr/README.md`.
