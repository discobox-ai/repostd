# repostd

The discobox-ai repository standard, and `repocheck`, the tool that enforces
it. The standard covers code layout, docs, agent config, tooling, lint, tests,
CI, and release.

The standard itself is the `repostd` skill:
[`internal/canon/files/managed/.agents/skills/repostd/SKILL.md.tmpl`](internal/canon/files/managed/.agents/skills/repostd/SKILL.md.tmpl).
Every conforming repo carries a synced copy at
`.agents/skills/repostd/SKILL.md`.

## Use

```sh
go get -tool github.com/discobox-ai/repostd/cmd/repocheck@main
go tool repocheck init    # new repo: write missing starter files, then sync
go tool repocheck         # report findings; exit 1 on unwaived errors
go tool repocheck sync    # rewrite managed files, the ADR index, symlinks
go tool repocheck rules   # list every rule
```

Waive a rule only where conforming is wrong for the repo, in
`.repocheck.yaml`:

```yaml
waivers:
  - rule: layout.go-work
    reason: the sandbox agent needs its own dependency set
```

Renovate: `renovate.json` extends `github>discobox-ai/repostd`, which is
[`default.json`](default.json) here.

## Build

```sh
go tool task build
go tool task ci
```
