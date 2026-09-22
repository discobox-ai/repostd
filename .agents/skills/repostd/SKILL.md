---
name: repostd
description: The discobox-ai repository standard for code layout, docs, agent config, tooling, lint, tests, CI, and release. Use when creating a discobox-ai repo, bringing one into conformance, reviewing a change that touches repo structure (new top-level directory, workflow, Taskfile target, lint config, ADR, DESIGN.md), or when `go tool repocheck` reports findings.
---

<!-- Managed by repostd (go tool repocheck sync). Do not edit here; change
     github.com/discobox-ai/repostd and sync. -->

# discobox-ai repository standard

Every `discobox-ai` repo follows this standard. Rules tagged with a rule ID
are enforced by `go tool repocheck` (run by `task check` and CI). Untagged
rules need judgment: apply them when writing and reviewing.

## Workflow

This skill is self-contained. It may be read from its URL,
`https://raw.githubusercontent.com/discobox-ai/repostd/main/.agents/skills/repostd/SKILL.md`,
without being installed, by any agent. It works on the git repository that
contains the current directory, whatever state that repository is in, even
empty. `$RC` finds the repo root itself. If the current directory is not
inside a git repository, `$RC` says so; stop and tell the user.

1. **Pick the runner, `$RC`.** Needs nothing but git and a Go toolchain.
   Use the first of these that applies:
   - `go.mod` has a `tool` directive for
     `github.com/discobox-ai/repostd/cmd/repocheck`: `$RC` is
     `go tool repocheck`.
   - `go` is on `PATH`: `$RC` is
     `env GOTOOLCHAIN=auto GOPROXY=direct go run github.com/discobox-ai/repostd/cmd/repocheck@main`.
     It downloads and builds on first use, installs nothing, and never
     edits the repo's `go.mod`. `GOTOOLCHAIN=auto` fetches a newer Go if
     the installed one is too old. `GOPROXY=direct` reads `main` straight
     from GitHub, because the module proxy serves a cached `@main` for a
     while after each push.
   - `nix` is on `PATH`: `$RC` is
     `nix shell nixpkgs#go -c env GOTOOLCHAIN=auto GOPROXY=direct go run github.com/discobox-ai/repostd/cmd/repocheck@main`.
   - Otherwise stop, and tell the user to install Go.
2. **New repo?** If the directory is empty or has no commits, this is a new
   repo. Run, in order, and then report as in step 5:
   - `git init -b main` when it is not a repo yet.
   - `go mod init github.com/discobox-ai/<repo>`, asking the user if the
     module path is not obvious. `$RC init` refuses to run without a
     `go.mod`, because the starter files carry the module path.
   - `$RC init`, which writes every starter and managed file and the
     symlinks.
   - `env GOPROXY=direct go get -tool` for
     `github.com/go-task/task/v3/cmd/task`,
     `github.com/golangci/golangci-lint/v2/cmd/golangci-lint` and
     `github.com/discobox-ai/repostd/cmd/repocheck@main`.
   - `go mod tidy`, because `go get -tool` leaves `go.sum` incomplete and
     `verify` would fail on it.
   - `nix flake lock`, then write `cmd/<repo>/main.go` and
     `internal/version/version.go`.
   - Fill in every TODO the starters leave: what the repo is, in `README.md`,
     `DESIGN.md` and `AGENTS.md`'s Project Structure.
   - `$RC` until clean, then `nix develop -c go tool task ci`, then commit.
   - Creating the GitHub repo and pushing is the user's call; ask.
3. **Check.** Run `$RC`. It prints one finding per line, in the form
   `path: severity [rule-id] message`, then a summary. It exits 1 when any
   unwaived error remains. `$RC rules` lists every rule.
4. **Review.** Read the untagged rules below against the repo; they are
   what `$RC` cannot check. Examples: packages in `pkg/` that nothing
   imports, a `DESIGN.md` describing planned work, workflows with logic
   beyond task calls.
5. **Report.** Group the findings by section (layout, docs, ADRs, agents,
   env, lint, tests, CI, release, git). For each group, say what is wrong
   and whether the fix is mechanical (step 6) or needs a decision. Stop
   here unless the user asked to fix or conform.
6. **Conform**, when asked:
   - With no `go.mod`, ask the user for the module path (normally
     `github.com/discobox-ai/<repo>`) and run `go mod init <path>`.
     `$RC init` refuses to run without one, because the starter files carry
     the module path.
   - Pin the tool:
     `env GOPROXY=direct go get -tool github.com/discobox-ai/repostd/cmd/repocheck@main`.
     From then on `$RC` is `go tool repocheck`.
   - Run `$RC init`. It writes the starter files that are missing, the
     managed files, and the symlinks, and never overwrites an existing
     file. Where the repo already has its own version of a starter file,
     `$RC show <path>` prints the starter filled in for this repo (for
     example `$RC show Taskfile.yml`). Merge what it adds into the repo's
     file by hand, such as Taskfile targets, flake packages or AGENTS.md
     sections, and fill in every TODO.
   - Run `$RC sync` whenever managed files or ADRs change.
   - Fix everything else by hand, following the rule text below. Run
     `nix flake lock` if the repo has a new flake.
   - Re-run `$RC` until it is clean, then run `go tool task ci`.
   - Anything that needs a decision goes to the user; don't guess. Moving
     packages, renumbering ADRs and deleting files all need a decision.

- **Waivers.** Add one to `.repocheck.yaml` only when conforming is wrong for
  this repo, not merely inconvenient. A waiver is a `rule`, an optional
  `path` glob, and a required `reason`. Tell the user about every new
  waiver.
- **Managed files** come from repostd: `.golangci.yml`,
  `.github/actions/task/action.yml`, `.github/actionlint.yaml`,
  `docs/adr/template.md`, and this skill. Change them upstream, never
  locally. The one exception is inside `.golangci.yml`'s
  `# repostd:local` blocks.

## 1. Code layout

- Root directories are only `cmd`, `pkg`, `internal`, `docs`, `test` and
  `scripts`, plus dot-directories. No Go packages at the root.
  `layout.root-dirs`, `layout.root-go`
- Binaries go in `cmd/<binary>`. Repo-only tools (release, codegen) go in
  `internal/cmd/<tool>`.
- Use one Go module. `go.work` needs a waiver, and every task then covers
  every module. `layout.go-work`
- `pkg/` holds only packages other programs import. Default to `internal/`.
- Generic helpers useful beyond one repo belong in
  `github.com/discobox-ai/x`.
- Generated code is committed as `*_gen.go` or under `gen/`, and is produced
  by `//go:generate` directives that `task generate` runs.
- No Makefile, no `tools.go`, no goreleaser. `layout.no-makefile`,
  `layout.no-tools-go`, `layout.no-goreleaser`

## 2. Docs

- `AGENTS.md` is the real file. `CLAUDE.md` is a symlink to `AGENTS.md`
  wherever either exists. `docs.agents-symlink`
- The root `AGENTS.md` has these `##` sections: Project Structure, Shared
  Libraries, Git Workflow, Commands, Implementation Quality, Package Design
  Docs, Architecture Decision Records. `docs.agents-sections`
- Required files: `README.md` (user-facing), `DESIGN.md`, `LICENSE`
  (Apache-2.0), `NOTICE` and `SECURITY.md`. `docs.required-files`,
  `docs.license`
- `DESIGN.md` and `REVIEW.md`:
  - They sit next to the code and are read from the root down; closer files
    override their parents.
  - A `REVIEW.md` needs a `DESIGN.md` beside it. `docs.review-design`
  - A `DESIGN.md` over 300 lines is a warning and over 600 lines is an
    error: split it into child packages' docs. `docs.design-length`
  - `DESIGN.md` describes current state only, never planned work. Parents
    link to children instead of repeating them. Prefer Mermaid for
    structure and flows. Keep them short, directive and scannable.
  - `REVIEW.md` is a bullet list of review rules and pitfalls.
- Every Mermaid block starts with a known diagram type. `docs.mermaid`
- No plan or task documents in the repo: plans belong in the branch or task.
  `docs.no-plans`

### ADRs (`docs/adr`)

- Filenames are `NNNN-slug-stating-the-decision.md`: four digits, unique,
  with a lowercase hyphenated slug. `adr.unique`
- The header is exactly:

  ```markdown
  # NNNN — Title
  - **Status**: Proposed | Accepted | Rejected | Superseded by NNNN
  - **Date**: YYYY-MM-DD
  - **Supersedes**: NNNN        (optional)
  ```

- "Superseded by B" in A and "Supersedes: A" in B always appear together.
  `adr.supersede`
- The sections are `## Context`, `## Decision`, `## Alternatives rejected`
  and `## Consequences`, plus `## Deferred` when something was deferred.
  `adr.format` checks the filename, header and sections.
- `docs/adr/README.md` holds the index between
  `<!-- repostd:adr-index -->` markers. `repocheck sync` generates it.
  `adr.files`, `adr.index`
- Write an ADR only when a plausible alternative was rejected for a
  non-obvious reason, or something was deferred with a condition for
  revisiting it. Otherwise update `DESIGN.md`.
- Land the ADR as `Proposed` before implementation; `Accepted` is the
  go-ahead. Amend it while nothing has shipped against it, and supersede it
  after.

## 3. Agent config

- Skills live in `.agents/skills/`. `.claude/skills` is a symlink to
  `../.agents/skills`. `agents.skills-symlink`
- `.claude/settings.local.json` is never committed.
  `agents.no-settings-local`
- `.discobox/hooks/*` call Taskfile targets and nothing else.

## 4. Environment and tools

- `flake.nix`, `flake.lock` and `.envrc` (`use flake`) are required. The
  flake's default devShell provides Go and every non-Go tool CI uses:
  shellcheck, actionlint, node, and so on. CI gets its environment only from
  the flake. `env.flake`, `env.envrc`
- Go-program tools are pinned by `tool` directives in `go.mod` and never also
  in the flake, because two pins drift. The minimum set is task,
  golangci-lint and repocheck. `env.go-tools`, `env.flake-go-tools`
- Always run task as `go tool task`. `Taskfile.yml` defines these targets
  (`env.taskfile-targets`):

  | Target | What it does |
  |---|---|
  | `build` | Builds into `build/`. |
  | `run` | Runs the program, where there is one. |
  | `dev` | Hot-reloads it through watchnbuild, where there is one. `env.dev-watch` |
  | `test` | `go test -race`. `env.test-race` |
  | `check` | golangci-lint, `repocheck`, shellcheck, actionlint. |
  | `fmt`, `tidy`, `generate` | Format, tidy go.mod, regenerate code. |
  | `verify` | fmt, tidy, generate and `repocheck sync` leave the tree unchanged. |
  | `ci` | `verify` + `check` + `test`. |

- A repo with a long-running program (a server, a daemon) has a `dev`
  target: hot reload through `go tool watchnbuild`, configured by
  `.wnb.yaml` at the root, with `.wnb.<name>.yaml` for a second loop such as
  a CLI built beside the server. Pin
  `github.com/discobox-ai/watchnbuild` as a tool. Every config is used by a
  dev target, and every dev target drives watchnbuild. `env.dev-watch`
- Logic beyond a few lines goes in a Go program under `internal/cmd`, not in
  Taskfile shell or workflow YAML. Shared programs go in repostd or `x`.

## 5. Lint

- `.golangci.yml` is managed. Repo additions go only inside the
  `# repostd:local` blocks. `managed.files`
- It bans testify with depguard, and `nolintlint` requires every `nolint` to
  be specific and explained.

## 6. Tests

- Use the standard `testing` package. testify is banned; go-cmp is allowed.
  `tests.no-testify`
- Integration and e2e tests are named `*_integration_test.go` or
  `*_e2e_test.go` and call `t.Skip` unless `<REPO>_INTEGRATION=1` or
  `<REPO>_E2E=1` is set. Build tags are only for platforms.
  `tests.no-integration-tags`
- Every `*_INTEGRATION` or `*_E2E` variable the Taskfile sets is read by a
  test. `tests.env-read`
- Test-helper packages are named `<pkg>test`. Test names read as sentences.

## 7. CI (GitHub Actions on Depot runners)

- `.github/workflows/ci.yml` runs on `pull_request` and on `push` to `main`,
  with jobs `verify`, `check` and `test`. Each job runs `actions/checkout`
  and then `./.github/actions/task` with its target. `ci.workflow`
- Every job runs on a Depot runner (`depot-ubuntu-*`, `depot-macos-*`,
  `depot-windows-*`). `ci.runners`
- Workflows install no tools: no `actions/setup-*`. Nix does not run on
  Windows, so a `depot-windows-*` job may use `actions/setup-go` with
  `go-version-file: go.mod` and nothing else. `ci.no-setup`
- Workflows contain no build logic. Every `run:` is one
  `[nix develop -c] go tool task <target>`. `ci.run-steps`
- Every `uses:` is pinned to a 40-character commit hash with a `# vX.Y.Z`
  comment. `ci.pinned`
- Top-level `permissions: contents: read`; jobs widen only what they need.
  `actions/checkout` sets `persist-credentials: false`. `ci.permissions`,
  `ci.checkout-credentials`
- Keep Windows CI where the repo ships for Windows.
- `renovate.json` extends `github>discobox-ai/repostd`, which covers gomod
  (including `tool` directives), action digests and `flake.lock`.
  `ci.renovate`

## 8. Release

- No goreleaser. `.github/workflows/release.yml` runs on `v*` tags and calls
  only `go tool task release:*` targets. `ci.release`
- Release logic (archives, checksums, manifests, images, the Homebrew tap)
  is written as Go programs: `internal/cmd` when it is specific to the repo,
  and a repostd `go tool` when it is shared.
- Tags are SemVer `vX.Y.Z`, with prereleases as `-alpha.N` or `-rc.N`. The
  version is set by ldflags into `internal/version` and falls back to the
  VCS revision.

## 9. Git

- `.gitattributes` starts with `* text=auto eol=lf`. `git.attributes`
- `.gitignore` covers `build/`, `.direnv/`, `.env` and `result`.
  `git.ignore`
- Commit messages are Conventional Commits: `feat`, `fix`, `refactor`,
  `docs`, `test`, `chore`, `perf` or `style`.
