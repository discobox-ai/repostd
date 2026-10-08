package rules

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/discobox-ai/repostd/internal/repo"
)

// RequiredScripts are the package.json scripts every node repo has: the
// Taskfile targets' names and meanings, run as `pnpm run <script>` (ADR 0002).
var RequiredScripts = []string{"test", "check", "fmt", "generate", "verify", "ci"}

// RequiredNodeTools are the development dependencies the managed oxlint and
// oxfmt configs, and the typecheck, need: Remix 3's own tools.
var RequiredNodeTools = []string{"oxlint", "oxlint-tsgolint", "oxfmt", "typescript"}

// OxlintBase is the managed oxlint config a repo's .oxlintrc.json extends.
const OxlintBase = "oxlint.repostd.json"

// foreignLockfiles are the lockfiles of package managers other than pnpm.
var foreignLockfiles = []string{"package-lock.json", "npm-shrinkwrap.json", "yarn.lock", "bun.lock", "bun.lockb"}

var (
	pnpmPinRE = regexp.MustCompile(`^pnpm@\d+\.\d+\.\d+(\+\S+)?$`)
	// scriptCallRE matches a script calling another through pnpm or npm:
	// `pnpm run x`, `pnpm x`, `pnpm -s x`, `npm run x`.
	scriptCallRE = regexp.MustCompile(`\b(?:pnpm(?:\s+(?:-s|--silent))?(?:\s+run)?|npm\s+run)\s+([A-Za-z0-9:_.-]+)`)
	oxlintRE     = regexp.MustCompile(`\boxlint\b`)
	oxfmtCheck   = regexp.MustCompile(`\boxfmt\b[^&|;]*\s--check\b`)
	oxfmtWrite   = regexp.MustCompile(`\boxfmt\b[^&|;]*\s--write\b`)
	// extendsBaseRE matches .oxlintrc.json extending the managed config.
	extendsBaseRE = regexp.MustCompile(`"extends"\s*:\s*\[[^\]]*"(?:\./)?oxlint\.repostd\.json"`)
	tscRE         = regexp.MustCompile(`\btsc\b`)
	doctorRE      = regexp.MustCompile(`\bremix\s+doctor\b[^&|;]*\s--strict\b`)
)

var nodeRules = []Rule{
	{
		ID:      "env.node-pins",
		Summary: "package.json pins pnpm in packageManager and node in devEngines.runtime; pnpm-lock.yaml is the only lockfile.",
		Stacks:  nodeOnly,
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			if pm := r.Package.PackageManager; !pnpmPinRE.MatchString(pm) {
				out = append(out, issue("package.json", "packageManager is %q; pin pnpm exactly, as pnpm@X.Y.Z", pm))
			}
			if !runtimePinsNode(r.Package.DevEngines.Runtime) {
				out = append(out, issue("package.json", "devEngines.runtime does not pin node; set {\"name\": \"node\", \"version\": \"X.Y.Z\", \"onFail\": \"download\"}"))
			}
			if !r.Has("pnpm-lock.yaml") {
				out = append(out, issue("pnpm-lock.yaml", "missing pnpm-lock.yaml; commit the lockfile"))
			}
			for _, f := range foreignLockfiles {
				if r.Has(f) {
					out = append(out, issue(f, "%s is another package manager's lockfile; a node repo uses pnpm", f))
				}
			}
			return out
		},
	},
	{
		ID:      "env.node-scripts",
		Summary: "package.json defines the standard scripts, and check, fmt and ci run what they must.",
		Stacks:  nodeOnly,
		Check: func(r *repo.Repo) []Issue {
			scripts := r.Package.Scripts
			var out []Issue
			for _, s := range RequiredScripts {
				if _, ok := scripts[s]; !ok {
					out = append(out, issue("package.json", "missing script %q", s))
				}
			}
			if _, ok := scripts["check"]; ok {
				check := scriptText(scripts, "check")
				for _, want := range []struct {
					re   *regexp.Regexp
					what string
				}{
					{oxlintRE, "oxlint"},
					{oxfmtCheck, "oxfmt --check"},
					{tscRE, "tsc"},
				} {
					if !want.re.MatchString(check) {
						out = append(out, issue("package.json", "the check script does not run %s", want.what))
					}
				}
				if r.Package.Depends("remix") && !doctorRE.MatchString(check) {
					out = append(out, issue("package.json", "the check script does not run `remix doctor --strict`, which checks Remix's conventions"))
				}
			}
			if _, ok := scripts["fmt"]; ok && !oxfmtWrite.MatchString(scriptText(scripts, "fmt")) {
				out = append(out, issue("package.json", "the fmt script does not run oxfmt --write"))
			}
			if _, ok := scripts["ci"]; ok {
				calls := scriptCalls(scripts, "ci")
				for _, s := range CIJobs {
					if !calls[s] {
						out = append(out, issue("package.json", "the ci script does not run %q", s))
					}
				}
			}
			return out
		},
	},
	{
		ID:      "env.node-tools",
		Summary: "package.json depends on oxlint, oxlint-tsgolint, oxfmt, and typescript, and .oxlintrc.json extends oxlint.repostd.json.",
		Stacks:  nodeOnly,
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, t := range RequiredNodeTools {
				if !r.Package.Depends(t) {
					out = append(out, issue("package.json", "missing development dependency %s (pnpm add -D %s)", t, t))
				}
			}
			data, ok := r.Read(".oxlintrc.json")
			switch {
			case !ok:
				out = append(out, issue(".oxlintrc.json", "missing .oxlintrc.json extending %s (run repocheck init)", OxlintBase))
			case !extendsBaseRE.Match(data):
				out = append(out, issue(".oxlintrc.json", "does not extend ./%s, so the standard's lint rules do not run", OxlintBase))
			}
			return out
		},
	},
}

// runtimePinsNode reports whether devEngines.runtime, an object or a list of
// them, names node with a version.
func runtimePinsNode(raw json.RawMessage) bool {
	type runtime struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	var list []runtime
	if err := json.Unmarshal(raw, &list); err != nil {
		var one runtime
		if json.Unmarshal(raw, &one) != nil {
			return false
		}
		list = []runtime{one}
	}
	return slices.ContainsFunc(list, func(rt runtime) bool { return rt.Name == "node" && rt.Version != "" })
}

// scriptCalls returns the scripts name runs, itself included, following
// calls through pnpm and npm.
func scriptCalls(scripts map[string]string, name string) map[string]bool {
	seen := map[string]bool{}
	var walk func(string)
	walk = func(n string) {
		body, ok := scripts[n]
		if !ok || seen[n] {
			return
		}
		seen[n] = true
		for _, m := range scriptCallRE.FindAllStringSubmatch(body, -1) {
			walk(m[1])
		}
	}
	walk(name)
	return seen
}

// scriptText is the text of name and every script it calls, so that a check
// split into check:lint and check:types is read as one.
func scriptText(scripts map[string]string, name string) string {
	var names []string
	for n := range scriptCalls(scripts, name) {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "%s\n", scripts[n])
	}
	return b.String()
}
