package rules_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/discobox-ai/repostd/internal/canon"
	"github.com/discobox-ai/repostd/internal/managed"
	"github.com/discobox-ai/repostd/internal/repo"
)

// packageJSON is a node repo's manifest that meets the standard: a Remix app
// on pnpm, with every standard script and tool.
const packageJSON = `{
  "name": "example",
  "private": true,
  "type": "module",
  "packageManager": "pnpm@11.9.0",
  "devEngines": {
    "runtime": { "name": "node", "version": "24.18.0", "onFail": "download" }
  },
  "scripts": {
    "test": "remix test",
    "check": "pnpm run check:lint && pnpm run check:types && remix doctor --strict",
    "check:lint": "oxlint . --max-warnings=0 && oxfmt . --check",
    "check:types": "tsc --noEmit",
    "fmt": "oxfmt . --write",
    "generate": "node scripts/generate.ts",
    "verify": "pnpm run fmt && pnpm run generate && git diff --exit-code",
    "ci": "pnpm run verify && pnpm run check && pnpm run test"
  },
  "dependencies": { "remix": "3.0.0" },
  "devDependencies": {
    "oxfmt": "0.72.0",
    "oxlint": "1.87.0",
    "oxlint-tsgolint": "7.0.2003",
    "typescript": "7.0.2"
  }
}
`

// conformingNode returns a node repo, a Remix app in Remix's own layout,
// that init has brought to the standard.
func conformingNode(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	write(t, dir, "package.json", packageJSON)
	write(t, dir, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	write(t, dir, "server.ts", "import './app/router.ts'\n")
	write(t, dir, "app/router.ts", "export const router = {}\n")
	write(t, dir, "app/actions/controller.tsx", "export default {}\n")
	write(t, dir, "app/ui/public/entry.ts", "export {}\n")

	starter, err := canon.Starter(repo.Node, canon.Vars{Module: "example", Name: "example", Year: 2026, RepocheckVersion: "v0.0.0-20261008000000-000000000000"})
	if err != nil {
		t.Fatal(err)
	}
	r := load(t, dir)
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

// nodeStarterWith returns the node starter file p with the first old
// replaced by new.
func nodeStarterWith(t *testing.T, p, old, replacement string) string {
	t.Helper()
	files, err := canon.Starter(repo.Node, canon.Vars{Module: "example", Name: "example", Year: 2026, RepocheckVersion: "main"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == p {
			if old != "" && !strings.Contains(string(f.Data), old) {
				t.Fatalf("node starter %s has no %q", p, old)
			}
			return strings.Replace(string(f.Data), old, replacement, 1)
		}
	}
	t.Fatalf("no node starter file %s", p)
	return ""
}

func TestAnInitializedNodeRepoConformsToTheStandard(t *testing.T) {
	dir := conformingNode(t)
	r := load(t, dir)
	if r.Stack != repo.Node {
		t.Fatalf("Stack = %q, want node for a package.json without a go.mod", r.Stack)
	}
	for _, id := range failing(t, dir) {
		t.Errorf("unexpected failing rule %s", id)
	}
}

// A Remix app's own layout is not the Go root-directory list, and none of the
// Go toolchain rules apply to a repo with no go.mod.
func TestTheGoToolchainRulesSkipANodeRepo(t *testing.T) {
	dir := conformingNode(t)
	write(t, dir, "mock/server.ts", "export {}\n")
	for _, id := range failing(t, dir) {
		if strings.HasPrefix(id, "layout.") || id == "env.go-tools" || id == "env.flake" || id == "env.taskfile-targets" {
			t.Errorf("Go rule %s applied to a node repo", id)
		}
	}
}

// A package.json beside a go.mod is a Go repo's frontend, not a second stack.
func TestAGoRepoWithAPackageJSONIsStillAGoRepo(t *testing.T) {
	dir := conforming(t)
	write(t, dir, "package.json", packageJSON)
	if got := load(t, dir).Stack; got != repo.Go {
		t.Errorf("Stack = %q, want go", got)
	}
}

func TestEachNodeViolationIsReportedByItsRule(t *testing.T) {
	pkg := func(old, replacement string) string {
		if !strings.Contains(packageJSON, old) {
			t.Fatalf("package.json fixture has no %q", old)
		}
		return strings.Replace(packageJSON, old, replacement, 1)
	}
	oxfmtWith := func(extra string) string {
		f, ok := canon.ManagedFile(repo.Node, "oxfmt.config.ts")
		if !ok {
			t.Fatal("no managed oxfmt.config.ts")
		}
		return string(f.Data) + extra
	}
	tests := []struct {
		name   string
		files  map[string]string
		remove []string
		want   string
	}{
		{name: "pnpm unpinned", files: map[string]string{"package.json": pkg(`"pnpm@11.9.0"`, `"pnpm@11"`)}, want: "env.node-pins"},
		{name: "npm as the package manager", files: map[string]string{"package.json": pkg(`"pnpm@11.9.0"`, `"npm@11.0.0"`)}, want: "env.node-pins"},
		{name: "node unpinned", files: map[string]string{"package.json": pkg(`"devEngines": {
    "runtime": { "name": "node", "version": "24.18.0", "onFail": "download" }
  },`, `"engines": { "node": ">=24" },`)}, want: "env.node-pins"},
		{name: "no pnpm lockfile", remove: []string{"pnpm-lock.yaml"}, want: "env.node-pins"},
		{name: "an npm lockfile", files: map[string]string{"package-lock.json": "{}\n"}, want: "env.node-pins"},
		{name: "no verify script", files: map[string]string{"package.json": pkg(`    "verify": "pnpm run fmt && pnpm run generate && git diff --exit-code",
`, "")}, want: "env.node-scripts"},
		{name: "a check without oxlint", files: map[string]string{"package.json": pkg(`"oxlint . --max-warnings=0 && oxfmt . --check"`, `"oxfmt . --check"`)}, want: "env.node-scripts"},
		{name: "a check without oxfmt's check", files: map[string]string{"package.json": pkg(`"oxlint . --max-warnings=0 && oxfmt . --check"`, `"oxlint . --max-warnings=0"`)}, want: "env.node-scripts"},
		{name: "a check without the typecheck", files: map[string]string{"package.json": pkg(`"tsc --noEmit"`, `"true"`)}, want: "env.node-scripts"},
		{name: "a Remix app without remix doctor", files: map[string]string{"package.json": pkg(` && remix doctor --strict`, ``)}, want: "env.node-scripts"},
		{name: "remix doctor without --strict", files: map[string]string{"package.json": pkg(`remix doctor --strict`, `remix doctor`)}, want: "env.node-scripts"},
		{name: "a fmt that only checks", files: map[string]string{"package.json": pkg(`"oxfmt . --write"`, `"oxfmt . --check"`)}, want: "env.node-scripts"},
		{name: "a ci that skips the tests", files: map[string]string{"package.json": pkg(` && pnpm run test"`, `"`)}, want: "env.node-scripts"},
		{name: "no type-aware lint backend", files: map[string]string{"package.json": pkg(`    "oxlint-tsgolint": "7.0.2003",
`, "")}, want: "env.node-tools"},
		{name: "an oxlint config that does not extend the standard", files: map[string]string{".oxlintrc.json": "{\n  \"rules\": {}\n}\n"}, want: "env.node-tools"},
		{name: "no oxlint config", remove: []string{".oxlintrc.json"}, want: "env.node-tools"},
		{name: "a hook running oxlint directly", files: map[string]string{".discobox/hooks/50-lint.sh": "#!/bin/bash\n#---\n# name: Lint\n# type: file\n# pattern: \"**/*.ts\"\n#---\n\noxlint .\n"}, want: "discobox.hooks"},
		{name: "a hook running a tool through pnpm exec", files: map[string]string{".discobox/hooks/50-lint.sh": "#!/bin/bash\n#---\n# name: Lint\n# type: file\n# pattern: \"**/*.ts\"\n#---\n\npnpm exec tsc --noEmit\n"}, want: "discobox.hooks"},
		{name: "a dev script with no service", files: map[string]string{"package.json": pkg(`    "test": "remix test",`, `    "dev": "node --watch server.ts",
    "test": "remix test",`)}, want: "discobox.dev-service"},
		{name: "a ci job through the task action", files: map[string]string{".github/workflows/ci.yml": nodeStarterWith(t, ".github/workflows/ci.yml", "uses: ./.github/actions/pnpm", "uses: ./.github/actions/task")}, want: "ci.workflow"},
		{name: "no repocheck job", files: map[string]string{".github/workflows/ci.yml": nodeStarterWith(t, ".github/workflows/ci.yml", "uses: ./.github/actions/repocheck", "uses: ./.github/actions/pnpm")}, want: "ci.workflow"},
		{name: "setup-node in a workflow", files: map[string]string{".github/workflows/ci.yml": nodeStarterWith(t, ".github/workflows/ci.yml", "      - uses: ./.github/actions/pnpm\n        with:\n          script: test\n", "      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020 # v7.0.0\n      - uses: ./.github/actions/pnpm\n        with:\n          script: test\n")}, want: "ci.no-setup"},
		{name: "a run step that is not a script", files: map[string]string{".github/workflows/ci.yml": nodeStarterWith(t, ".github/workflows/ci.yml", "      - uses: ./.github/actions/pnpm\n        with:\n          script: test\n", "      - run: npm test\n")}, want: "ci.run-steps"},
		{name: "hand-edited standard lint rules", files: map[string]string{"oxlint.repostd.json": "{}\n"}, want: "managed.files"},
		{name: "an oxfmt local block the standard dropped", files: map[string]string{"oxfmt.config.ts": oxfmtWith("// repostd:local gone\n// a line\n// repostd:end\n")}, want: "managed.files"},
		{name: "node_modules not ignored", files: map[string]string{".gitignore": ".env\n"}, want: "git.ignore"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := conformingNode(t)
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

func TestANodeDevScriptWithItsServicePasses(t *testing.T) {
	dir := conformingNode(t)
	write(t, dir, "package.json", strings.Replace(packageJSON, `    "test": "remix test",`, "    \"dev\": \"node --watch server.ts\",\n    \"test\": \"remix test\",", 1))
	write(t, dir, ".discobox/services/10-app.sh", "#!/bin/bash\n#---\n# name: App\n# description: the dev loop\n#---\n\nexec pnpm run dev\n")
	for _, id := range failing(t, dir) {
		if strings.HasPrefix(id, "discobox.") {
			t.Errorf("failing rule %s; this dev loop conforms", id)
		}
	}
}

func TestSyncKeepsAnOxfmtBlockTheStandardDropped(t *testing.T) {
	dir := conformingNode(t)
	f, _ := canon.ManagedFile(repo.Node, "oxfmt.config.ts")
	edited := string(f.Data) + "// repostd:local gone\n// a line\n// repostd:end\n"
	write(t, dir, "oxfmt.config.ts", edited)
	_, err := managed.Sync(load(t, dir))
	if err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("Sync error = %v, want one naming block gone", err)
	}
	if got, _ := load(t, dir).Read("oxfmt.config.ts"); string(got) != edited {
		t.Errorf("Sync rewrote oxfmt.config.ts and dropped the block:\n%s", got)
	}
}
