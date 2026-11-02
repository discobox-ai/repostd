# 26-11-02-818 — Go CI caches through Depot Cache, and only pull requests and branch pushes share it

- **Status**: Accepted
- **Date**: 2026-11-02

## Context

The managed Go task action restored and saved `~/.cache/go-build` and
`~/go/pkg/mod` with `actions/cache`, on the reasoning that Depot runners back
the Actions cache API with Depot Cache, so `actions/cache` was "already the
fast path".

Measured in discobox-ai/discobox#443, it was not. `actions/cache` moves the
whole cache as one tarball: on that repo's main, restoring and saving it cost
6–11 minutes a job (a 12.8 GB upload on `check`), and on Depot's macOS runner
it moved about 30 MB/s. Meanwhile Depot's Linux and macOS runners already
start every job with `GOCACHEPROG` pointing at Depot Cache (`depot gocache`),
which Go reads and writes per object, and with `GOLANGCI_LINT_CACHEPROG`
beside it. With no `actions/cache` at all, modules fetched from the proxy each
run, jobs ran 3–7× faster. Depot's Windows runner carries Depot Cache's
credentials but neither the CLI nor `GOCACHEPROG`; installing the CLI and
setting it served 155 test packages from cache and halved the job.

A shared remote build cache is also a trust boundary. A pull request's test
code runs with the cache's credentials and can write entries, and Go trusts
whatever the cache returns for a key, so a build that reads it can link what a
pull request planted.

## Decision

1. **No `actions/cache` in a Go repo.** Go's build and test cache is Depot
   Cache through the runner's `GOCACHEPROG`; modules come from the module
   proxy. `actions/setup-go` sets `cache: false`, since its cache is
   `actions/cache`. `ci.no-actions-cache` enforces both.
2. **Windows sets it itself.** The Go starter's Windows job installs the Depot
   CLI (`depot/setup-action`) and sets `GOCACHEPROG: depot gocache` on its
   test step.
3. **Only `pull_request` events and pushes to a branch share the cache.**
   Every other run — a tag that releases, a release dispatched by hand (which
   runs off a branch), a schedule, a trigger added later — clears
   `GOCACHEPROG` and `GOLANGCI_LINT_CACHEPROG`, and the managed action fails
   the job if Go still sees one. Trust is decided by event, not by which job
   publishes, so a new publishing path is off the cache without anyone
   remembering to opt it out.

## Alternatives rejected

- **Keep `actions/cache`, as Depot backs it.** Depot makes the transfer fast,
  not small: the tarball is still restored before and saved after every job,
  and it duplicates what `GOCACHEPROG` already holds.
- **`actions/setup-go`'s built-in cache.** It is `actions/cache` under another
  name, with the same whole-tarball cost.
- **Keep a module cache in `actions/cache`.** Every run measured without one
  was no slower; restoring it would cost about what fetching the modules does.
- **Turn the shared cache off per publishing job.** An opt-out a new release
  path has to remember; a `workflow_dispatch` release runs off a branch, so a
  ref check alone misses it too.

## Consequences

- Go jobs spend nothing on cache restore or save, and pull requests reuse
  each other's and main's compiled packages and test results.
- Release builds compile from cold local caches.
- `main`'s CI reads entries pull requests wrote; it publishes nothing, so
  that is accepted.
- Every job depends on the module proxy being reachable.
- Node repos are unchanged: Depot Cache has no pnpm integration, so the
  managed pnpm action keeps its store cache.
