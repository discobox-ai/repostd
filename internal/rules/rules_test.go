package rules_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/discobox-ai/repostd/internal/canon"
	"github.com/discobox-ai/repostd/internal/managed"
	"github.com/discobox-ai/repostd/internal/repo"
	"github.com/discobox-ai/repostd/internal/rules"
)

const goMod = `module github.com/discobox-ai/example

go 1.27.1

tool (
	github.com/discobox-ai/repostd/cmd/repocheck
	github.com/go-task/task/v3/cmd/task
	github.com/golangci/golangci-lint/v2/cmd/golangci-lint
)
`

// conforming returns a git repo that init has brought to the standard.
func conforming(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	write(t, dir, "go.mod", goMod)
	write(t, dir, "flake.lock", "{}\n")
	write(t, dir, "cmd/example/main.go", "package main\n\nfunc main() {}\n")

	r := load(t, dir)
	starter, err := canon.Starter(canon.Vars{Module: "github.com/discobox-ai/example", Name: "example", Year: 2026})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range starter {
		if err := r.Write(f.Path, f.Data); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := managed.Sync(load(t, dir)); err != nil {
		if strings.Contains(err.Error(), "symlink") {
			t.Skipf("symlinks unavailable: %v", err)
		}
		t.Fatal(err)
	}
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, dir, p, data string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func load(t *testing.T, dir string) *repo.Repo {
	t.Helper()
	r, err := repo.Load(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// failing returns the IDs of rules with unwaived findings in dir.
func failing(t *testing.T, dir string) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, f := range rules.Run(load(t, dir)) {
		if !f.Waived() && f.Severity == rules.Error {
			seen[f.Rule] = true
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func TestAnInitializedRepoConformsToTheStandard(t *testing.T) {
	dir := conforming(t)
	for _, f := range rules.Run(load(t, dir)) {
		t.Errorf("unexpected finding: %s", f)
	}
}

func TestEachViolationIsReportedByItsRule(t *testing.T) {
	adr := func(n, title, status, extra string) string {
		return "# " + n + " — " + title + "\n\n- **Status**: " + status + "\n- **Date**: 2026-09-22\n" + extra +
			"\n## Context\n\nc\n\n## Decision\n\nd\n\n## Alternatives rejected\n\na\n\n## Consequences\n\nc\n"
	}
	tests := []struct {
		name  string
		files map[string]string
		// remove lists paths to delete from the conforming repo.
		remove []string
		want   string
	}{
		{name: "a package directory at the root", files: map[string]string{"proxy/proxy.go": "package proxy\n"}, want: "layout.root-dirs"},
		{name: "a Go file at the root", files: map[string]string{"generate.go": "package example\n"}, want: "layout.root-go"},
		{name: "a go.work", files: map[string]string{"go.work": "go 1.27.1\n"}, want: "layout.go-work"},
		{name: "a Makefile", files: map[string]string{"Makefile": "all:\n"}, want: "layout.no-makefile"},
		{name: "a tools.go", files: map[string]string{"internal/tools/tools.go": "//go:build tools\n\npackage tools\n"}, want: "layout.no-tools-go"},
		{name: "a goreleaser config", files: map[string]string{".goreleaser.yaml": "version: 2\n"}, want: "layout.no-goreleaser"},
		{name: "a regular CLAUDE.md", remove: []string{"CLAUDE.md"}, files: map[string]string{"CLAUDE.md": "# hi\n"}, want: "docs.agents-symlink"},
		{name: "an AGENTS.md without its sections", files: map[string]string{"AGENTS.md": "# Repository Guidelines\n"}, want: "docs.agents-sections"},
		{name: "no SECURITY.md", remove: []string{"SECURITY.md"}, want: "docs.required-files"},
		{name: "a non-Apache LICENSE", files: map[string]string{"LICENSE": "MIT License\n"}, want: "docs.license"},
		{name: "a REVIEW.md without a DESIGN.md", files: map[string]string{"internal/x/REVIEW.md": "- rule\n"}, want: "docs.review-design"},
		{name: "a 700-line DESIGN.md", files: map[string]string{"DESIGN.md": strings.Repeat("line\n", 700)}, want: "docs.design-length"},
		{name: "a plan document", files: map[string]string{"docs/tasks/plan.md": "- [ ] do it\n"}, want: "docs.no-plans"},
		{name: "a mermaid block with no diagram type", files: map[string]string{"docs/x.md": "```mermaid\nA --> B\n```\n"}, want: "docs.mermaid"},
		{name: "an ADR with a dated name", files: map[string]string{"docs/adr/2026-09-22-x.md": adr("0001", "X", "Accepted", "")}, want: "adr.format"},
		{name: "an ADR with an unknown status", files: map[string]string{"docs/adr/0001-x.md": adr("0001", "X", "Done", "")}, want: "adr.format"},
		{name: "an ADR missing Alternatives rejected", files: map[string]string{"docs/adr/0001-x.md": strings.Replace(adr("0001", "X", "Accepted", ""), "## Alternatives rejected", "## Alternatives", 1)}, want: "adr.format"},
		{name: "two ADRs with one number", files: map[string]string{"docs/adr/0001-x.md": adr("0001", "X", "Accepted", ""), "docs/adr/0001-y.md": adr("0001", "Y", "Accepted", "")}, want: "adr.unique"},
		{name: "a one-way supersede", files: map[string]string{"docs/adr/0001-x.md": adr("0001", "X", "Superseded by 0002", ""), "docs/adr/0002-y.md": adr("0002", "Y", "Accepted", "")}, want: "adr.supersede"},
		{name: "a supersede of a missing ADR", files: map[string]string{"docs/adr/0001-x.md": adr("0001", "X", "Superseded by 0009", "")}, want: "adr.supersede"},
		{name: "a stale ADR index", files: map[string]string{"docs/adr/0001-x.md": adr("0001", "X", "Accepted", "")}, want: "adr.index"},
		{name: "a real .claude/skills directory", remove: []string{".claude/skills"}, files: map[string]string{".claude/skills/x/SKILL.md": "x\n"}, want: "agents.skills-symlink"},
		{name: "committed per-user settings", files: map[string]string{".claude/settings.local.json": "{}\n"}, want: "agents.no-settings-local"},
		{name: "no flake.lock", remove: []string{"flake.lock"}, want: "env.flake"},
		{name: "an .envrc without the flake", files: map[string]string{".envrc": "dotenv\n"}, want: "env.envrc"},
		{name: "no repocheck tool directive", files: map[string]string{"go.mod": strings.Replace(goMod, "\tgithub.com/discobox-ai/repostd/cmd/repocheck\n", "", 1)}, want: "env.go-tools"},
		{name: "golangci-lint in the flake", files: map[string]string{"flake.nix": "{ packages = [ pkgs.golangci-lint ]; }\n"}, want: "env.flake-go-tools"},
		{name: "a Taskfile without ci", files: map[string]string{"Taskfile.yml": "version: \"3\"\ntasks:\n  build: {cmds: [go build]}\n"}, want: "env.taskfile-targets"},
		{name: "tests without -race", files: map[string]string{"Taskfile.yml": starterWith(t, "Taskfile.yml", "{{.RACE}} ", "")}, want: "env.test-race"},
		{name: "a hand-edited golangci config", files: map[string]string{".golangci.yml": "version: \"2\"\n"}, want: "managed.files"},
		{name: "testify in go.mod", files: map[string]string{"go.mod": goMod + "\nrequire github.com/stretchr/testify v1.10.0\n"}, want: "tests.no-testify"},
		{name: "an integration build tag", files: map[string]string{"internal/x/x_test.go": "//go:build integration\n\npackage x\n"}, want: "tests.no-integration-tags"},
		{name: "a test env var no test reads", files: map[string]string{"Taskfile.yml": starterWith(t, "Taskfile.yml", "  test:\n", "  test:e2e:\n    env: {EXAMPLE_E2E: \"1\"}\n    cmds: [go test ./...]\n\n  test:\n")}, want: "tests.env-read"},
		{name: "a test env var that only appears as data", files: map[string]string{
			"Taskfile.yml":         starterWith(t, "Taskfile.yml", "  test:\n", "  test:e2e:\n    cmds: [EXAMPLE_E2E=1 go test ./...]\n\n  test:\n"),
			"internal/x/x_test.go": "package x\n\nvar env = map[string]string{\"EXAMPLE_E2E\": \"true\"}\n",
		}, want: "tests.env-read"},
		{name: "a GitHub-hosted runner in matrix.include", files: map[string]string{".github/workflows/build.yml": "on: push\npermissions:\n  contents: read\njobs:\n  build:\n    strategy:\n      matrix:\n        include:\n          - runs-on: depot-ubuntu-24.04-4\n          - runs-on: macos-15\n    runs-on: ${{ matrix.runs-on }}\n    steps:\n      - run: go tool task build\n"}, want: "ci.runners"},
		{name: "a dev target without watchnbuild", files: map[string]string{
			"Taskfile.yml": starterWith(t, "Taskfile.yml", "  test:\n", "  dev:\n    cmds: [go run ./cmd/example]\n\n  test:\n"),
			".wnb.yaml":    "build:\n  command: go build ./...\n",
		}, want: "env.dev-watch"},
		{name: "an orphaned watchnbuild config", files: map[string]string{".wnb.yaml": "build:\n  command: go build ./...\n"}, want: "env.dev-watch"},
		{name: "a dev target using a config that does not exist", files: map[string]string{
			"Taskfile.yml": starterWith(t, "Taskfile.yml", "  test:\n", "  dev:cli:\n    cmds: [go tool watchnbuild -config .wnb.cli.yaml]\n\n  test:\n"),
			".wnb.yaml":    "build:\n  command: go build ./...\n",
			"go.mod":       strings.Replace(goMod, "\tgithub.com/go-task", "\tgithub.com/discobox-ai/watchnbuild\n\tgithub.com/go-task", 1),
		}, want: "env.dev-watch"},
		{name: "a watchnbuild loop without the tool pinned", files: map[string]string{
			"Taskfile.yml": starterWith(t, "Taskfile.yml", "  test:\n", "  dev:\n    cmds: [go tool watchnbuild]\n\n  test:\n"),
			".wnb.yaml":    "build:\n  command: go build ./...\n",
		}, want: "env.dev-watch"},
		{name: "a job on a GitHub-hosted runner", files: map[string]string{".github/workflows/ci.yml": starterWith(t, ".github/workflows/ci.yml", "depot-ubuntu-24.04-4", "ubuntu-latest")}, want: "ci.runners"},
		{name: "setup-go on Linux", files: map[string]string{".github/workflows/ci.yml": starterWith(t, ".github/workflows/ci.yml", "depot-windows-2025-4", "depot-ubuntu-24.04-4")}, want: "ci.no-setup"},
		{name: "build logic in a workflow", files: map[string]string{".github/workflows/ci.yml": starterWith(t, ".github/workflows/ci.yml", "run: go tool task test", "run: go test ./...")}, want: "ci.run-steps"},
		{name: "an action pinned by tag", files: map[string]string{".github/workflows/ci.yml": strings.ReplaceAll(starterWith(t, ".github/workflows/ci.yml", "", ""), "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1", "actions/checkout@v7")}, want: "ci.pinned"},
		{name: "write permissions at the top level", files: map[string]string{".github/workflows/ci.yml": starterWith(t, ".github/workflows/ci.yml", "contents: read", "contents: write")}, want: "ci.permissions"},
		{name: "persisted checkout credentials", files: map[string]string{".github/workflows/ci.yml": strings.ReplaceAll(starterWith(t, ".github/workflows/ci.yml", "", ""), "persist-credentials: false", "persist-credentials: true")}, want: "ci.checkout-credentials"},
		{name: "a missing ci job", files: map[string]string{".github/workflows/ci.yml": starterWith(t, ".github/workflows/ci.yml", "target: verify", "target: fmt")}, want: "ci.workflow"},
		{name: "a release running a non-release target", files: map[string]string{".github/workflows/release.yml": "on:\n  push:\n    tags: [\"v*\"]\npermissions:\n  contents: read\njobs:\n  release:\n    runs-on: depot-ubuntu-24.04-4\n    steps:\n      - run: go tool task build\n"}, want: "ci.release"},
		{name: "a renovate config without the preset", files: map[string]string{"renovate.json": "{\"extends\": [\"config:recommended\"]}\n"}, want: "ci.renovate"},
		{name: "CRLF line endings", files: map[string]string{".gitattributes": "* text=auto\n"}, want: "git.attributes"},
		{name: "an unignored build directory", files: map[string]string{".gitignore": ".direnv/\n.env\nresult\n"}, want: "git.ignore"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := conforming(t)
			for _, p := range tt.remove {
				if err := os.RemoveAll(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
					t.Fatal(err)
				}
			}
			for p, data := range tt.files {
				write(t, dir, p, data)
			}
			if got := failing(t, dir); !slices.Contains(got, tt.want) {
				t.Errorf("failing rules = %v, want %s among them", got, tt.want)
			}
		})
	}
}

// starterWith returns the starter file p with the first old replaced by new.
func starterWith(t *testing.T, p, old, replacement string) string {
	t.Helper()
	files, err := canon.Starter(canon.Vars{Module: "github.com/discobox-ai/example", Name: "example", Year: 2026})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == p {
			s := string(f.Data)
			if old != "" && !strings.Contains(s, old) {
				t.Fatalf("starter %s has no %q", p, old)
			}
			return strings.Replace(s, old, replacement, 1)
		}
	}
	t.Fatalf("no starter file %s", p)
	return ""
}

func TestAWaiverSilencesOnlyItsRuleAndPath(t *testing.T) {
	dir := conforming(t)
	write(t, dir, "proxy/proxy.go", "package proxy\n")
	write(t, dir, "api/api.go", "package api\n")
	write(t, dir, repo.WaiverFile, "waivers:\n  - rule: layout.root-dirs\n    path: proxy\n    reason: migrating later\n")

	var unwaived []string
	for _, f := range rules.Run(load(t, dir)) {
		if !f.Waived() {
			unwaived = append(unwaived, f.Path)
		}
	}
	if !slices.Equal(unwaived, []string{"api"}) {
		t.Errorf("unwaived finding paths = %v, want [api]", unwaived)
	}
}

func TestAWaiverWithoutAReasonIsRejected(t *testing.T) {
	dir := conforming(t)
	write(t, dir, repo.WaiverFile, "waivers:\n  - rule: layout.go-work\n")
	if _, err := repo.Load(t.Context(), dir); err == nil {
		t.Error("Load accepted a waiver without a reason")
	}
}

var citedRE = regexp.MustCompile("`((?:layout|docs|adr|agents|env|managed|tests|ci|git)\\.[a-z-]+)`")

func TestTheSkillCitesExactlyTheRulesThatExist(t *testing.T) {
	skill, ok := canon.ManagedFile(".agents/skills/repostd/SKILL.md")
	if !ok {
		t.Fatal("no managed skill")
	}
	cited := map[string]bool{}
	for _, m := range citedRE.FindAllStringSubmatch(string(skill.Data), -1) {
		cited[m[1]] = true
	}
	exist := map[string]bool{}
	for _, r := range rules.All() {
		exist[r.ID] = true
		if !cited[r.ID] {
			t.Errorf("rule %s is not cited in the skill", r.ID)
		}
	}
	for id := range cited {
		if !exist[id] {
			t.Errorf("the skill cites %s, which is not a rule", id)
		}
	}
}

func TestAnEnvVarReadThroughAConstantCounts(t *testing.T) {
	dir := conforming(t)
	write(t, dir, "Taskfile.yml", starterWith(t, "Taskfile.yml", "  test:\n", "  test:e2e:\n    cmds: [EXAMPLE_E2E=1 go test ./...]\n\n  test:\n"))
	write(t, dir, "internal/x/x_e2e_test.go", "package x\n\nconst e2eEnv = \"EXAMPLE_E2E\"\n")
	if got := failing(t, dir); slices.Contains(got, "tests.env-read") {
		t.Errorf("failing rules = %v; a const-declared env var is a read", got)
	}
}

func TestLoadFromASubdirectoryChecksTheWholeRepo(t *testing.T) {
	dir := conforming(t)
	r, err := repo.Load(t.Context(), filepath.Join(dir, "cmd", "example"))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Has("AGENTS.md") {
		t.Errorf("Files = %v, want paths relative to the repo root", r.Files)
	}
}

func TestLoadOutsideAGitRepositoryFailsClearly(t *testing.T) {
	_, err := repo.Load(t.Context(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("err = %v, want \"not inside a git repository\"", err)
	}
}
