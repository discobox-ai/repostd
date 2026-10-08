# 0002 — A node repo follows the standard with its own tools

- **Status**: Accepted
- **Date**: 2026-10-08

## Context

[ADR 0001](0001-one-standard-enforced-by-repocheck-and-a-skill.md) wrote
the standard for Go repos: tools pinned by `go.mod` `tool` directives,
targets in a `Taskfile.yml` run as `go tool task`, golangci-lint, a Nix
flake as CI's only environment, and a root of `cmd`, `pkg`, `internal`,
`docs`, `test` and `scripts`. discobot, the first discobox-ai repo with no
Go in it (a Remix 3 UI on node and pnpm), fails 27 rules under it, almost
all for not being Go: no `go.mod`, no Taskfile, no flake, and the `app/`
and `public/` directories Remix requires. Meeting them would mean a
`go.mod` that exists only to pin a task runner, and Go and Nix in a repo
that otherwise needs neither. Waiving them would leave a node repo with no
standard for its tools, lint, CI or hooks at all.

## Decision

The standard has two stacks, Go and node. Everything that is not about the
toolchain — docs, ADRs, agent config, git, tests' naming, CI's shape and
pinning, discobox services — is one rule set for both. The toolchain rules
are per stack.

- **Detection.** A repo with a `go.mod` is a Go repo. A repo with a
  `package.json` and no `go.mod` is a node repo. Each toolchain rule
  applies to its own stack and reports nothing for the other.
- **Package manager and runtime.** A node repo uses pnpm. `package.json`
  pins pnpm in `packageManager` and node in `engines.node`; the lockfile
  is committed. These pins are what CI and a developer install from.
- **Targets are `package.json` scripts**, run as `pnpm run <script>`, with
  the same names and meaning as the Taskfile targets: `build`, `run`
  and `dev` where there is a program, `test`, `check`, `fmt`, `generate`,
  `verify` and `ci`. There is no Taskfile.
- **Lint and format are ESLint and Prettier**, with configs managed by
  repostd the way `.golangci.yml` is, and repo additions only in marked
  local blocks. `check` runs ESLint, Prettier's check, and the
  typechecker.
- **UI repos are Remix 3.** A node repo with a UI uses Remix 3 and its
  documented layout (`server.ts`, `app/` with `routes.ts`, `router.ts`,
  `actions/`, `ui/`, and `public/` beside its narrowest owner), and its
  `check` runs `remix doctor --strict`, which checks Remix's own
  conventions. `layout.root-dirs` and the other Go layout rules apply to Go
  repos only.
- **CI** keeps the `verify`, `check` and `test` jobs, Depot runners, pinned
  actions and read-only permissions. A node repo's jobs install node and
  pnpm through pinned setup actions from the versions `package.json` pins,
  and run only `pnpm run <script>` steps. One `repocheck` job runs
  repocheck with `actions/setup-go` and `go run` at a pinned version, the
  only Go a node repo's CI uses.
- **Discobox hooks and services** invoke `pnpm run <script>` instead of
  `go tool task <target>`, and never run a lint, format or test tool
  directly.
- This supersedes ADR 0001's toolchain choices for node repos only:
  `go tool task`, `go.mod` tool pins, Nix as CI's environment, and the root
  directory list. For Go repos ADR 0001 is unchanged.

## Alternatives rejected

- **Go tooling in node repos.** A `go.mod` for task and repocheck and a
  flake for node would satisfy every rule unchanged, but adds two
  toolchains to repos whose developers use neither, and a node repo's own
  scripts would still be the real targets behind a Taskfile wrapper.
- **Waivers per node repo.** Each would waive the same dozen rules with the
  same reason, and nothing would standardize what a node repo uses
  instead: every node repo would choose its own linter, scripts and CI.
- **Declaring the stack in `.repocheck.yaml`.** Explicit, but the files
  already say it unambiguously, and a declaration can disagree with them.
  A repo with both `go.mod` and `package.json` is a Go repo; a node
  frontend inside a Go repo is a judgment rule, not a second stack.
- **Biome, or typechecking alone.** Biome is one fast tool for both, but
  ESLint and Prettier are the established pair, with the widest plugin and
  editor support. Typechecking alone standardizes no style.
- **A checked list of node root directories.** Every framework convention
  would need a repostd change; `remix doctor` already checks the one
  framework we use, against its own documentation.
- **A prebuilt repocheck binary for node CI.** No Go in a node repo at all,
  but repostd would need a release pipeline before any node repo could be
  checked. One `setup-go` job costs less.

## Consequences

- repocheck learns the node stack: rules for `package.json` pins and
  scripts, the managed ESLint and Prettier configs, node CI steps, and
  node hooks; `init` writes node starter files for a node repo; the skill
  gains the node toolchain rules beside the Go ones.
- A node repo's developers need only node and pnpm. Its CI needs Go for
  one job.
- Choosing another UI framework, linter or package manager for a node repo
  is a change to this standard, not a waiver.
