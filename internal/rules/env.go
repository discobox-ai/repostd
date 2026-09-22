package rules

import (
	"errors"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/discobox-ai/repostd/internal/repo"
)

// RequiredTools are the go.mod tool directives every repo has.
var RequiredTools = []string{
	"github.com/go-task/task/v3/cmd/task",
	"github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
	"github.com/discobox-ai/repostd/cmd/repocheck",
}

// RequiredTargets are the Taskfile targets every repo has.
var RequiredTargets = []string{"build", "test", "check", "fmt", "tidy", "generate", "verify", "ci"}

// WatchTool is the hot-reload loop a `dev` target runs.
const WatchTool = "github.com/discobox-ai/watchnbuild"

// wnbConfigRE matches a watchnbuild config name: .wnb.yaml, or .wnb.<name>.yaml
// for a second loop.
var wnbConfigRE = regexp.MustCompile(`\.wnb(?:\.[a-z0-9-]+)?\.yaml`)

// repocheckRunRE matches a Taskfile command running repocheck's checks.
var repocheckRunRE = regexp.MustCompile(`(?m)go tool repocheck(\s+check)?\s*$`)

// flakeGoTools matches nixpkgs attributes for tools go.mod pins instead.
var flakeGoTools = regexp.MustCompile(`\bpkgs\.(go-task|golangci-lint|ogen|gopls)\b`)

var envRules = []Rule{
	{
		ID:      "env.flake",
		Summary: "flake.nix and flake.lock exist.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, f := range []string{"flake.nix", "flake.lock"} {
				if !r.Has(f) {
					out = append(out, issue(f, "missing %s; CI and dev get every non-Go tool from the flake", f))
				}
			}
			return out
		},
	},
	{
		ID:      "env.envrc",
		Summary: ".envrc uses the flake.",
		Check: func(r *repo.Repo) []Issue {
			data, ok := r.Read(".envrc")
			if !ok {
				return []Issue{issue(".envrc", "missing .envrc with `use flake`")}
			}
			for l := range strings.Lines(string(data)) {
				if strings.TrimSpace(l) == "use flake" {
					return nil
				}
			}
			return []Issue{issue(".envrc", "no `use flake` line")}
		},
	},
	{
		ID:      "env.go-tools",
		Summary: "go.mod pins task, golangci-lint, and repocheck as tool directives.",
		Check: func(r *repo.Repo) []Issue {
			if r.Mod == nil {
				return []Issue{issue("go.mod", "missing go.mod")}
			}
			have := map[string]bool{}
			for _, t := range r.Mod.Tool {
				have[t.Path] = true
			}
			var out []Issue
			for _, t := range RequiredTools {
				if !have[t] {
					out = append(out, issue("go.mod", "missing tool directive %s (go get -tool %s)", t, t))
				}
			}
			return out
		},
	},
	{
		ID:      "env.flake-go-tools",
		Summary: "The flake does not also pin Go tools that go.mod pins.",
		Check: func(r *repo.Repo) []Issue {
			data, ok := r.Read("flake.nix")
			if !ok {
				return nil
			}
			var out []Issue
			for _, m := range flakeGoTools.FindAllStringSubmatch(string(data), -1) {
				out = append(out, issue("flake.nix", "pkgs.%s: Go tools are pinned by go.mod tool directives only", m[1]))
			}
			return out
		},
	},
	{
		ID:      "env.taskfile-targets",
		Summary: "Taskfile.yml defines the standard targets and runs repocheck.",
		Check: func(r *repo.Repo) []Issue {
			tasks, raw, err := taskfile(r)
			if err != nil {
				return []Issue{issue("Taskfile.yml", "%v", err)}
			}
			var out []Issue
			for _, t := range RequiredTargets {
				if _, ok := tasks[t]; !ok {
					out = append(out, issue("Taskfile.yml", "missing target %q", t))
				}
			}
			if !strings.Contains(raw, "go tool repocheck sync") {
				out = append(out, issue("Taskfile.yml", "no target runs `go tool repocheck sync` (verify must)"))
			}
			if !repocheckRunRE.MatchString(raw) {
				out = append(out, issue("Taskfile.yml", "no target runs `go tool repocheck` (check must)"))
			}
			return out
		},
	},
	{
		ID:      "env.dev-watch",
		Summary: "Where there is a dev target, hot reload is watchnbuild against a .wnb.yaml, and no config is orphaned.",
		Check: func(r *repo.Repo) []Issue {
			tasks, raw, err := taskfile(r)
			if err != nil {
				return nil // env.taskfile-targets reports it.
			}
			hasDev := false
			for name := range tasks {
				hasDev = hasDev || name == "dev" || strings.HasPrefix(name, "dev:")
			}
			configs := r.Match(".wnb*.yaml")
			if !hasDev && len(configs) == 0 {
				return nil // A repo with no long-running program needs neither.
			}

			var out []Issue
			// A repo may have helper dev:* targets that do not hot-reload, so
			// this asks only that the loop exists and is wired up.
			switch {
			case !hasDev:
				out = append(out, issue(configs[0], "watchnbuild config without a dev target"))
			case !strings.Contains(raw, "watchnbuild"):
				out = append(out, issue("Taskfile.yml", "no dev target runs watchnbuild; hot reload is `go tool watchnbuild -config <file>`"))
			case !r.Has(".wnb.yaml"):
				out = append(out, issue(".wnb.yaml", "dev target without a .wnb.yaml"))
			}
			if hasDev && r.Mod != nil {
				pinned := false
				for _, t := range r.Mod.Tool {
					pinned = pinned || t.Path == WatchTool
				}
				if !pinned && strings.Contains(raw, "watchnbuild") {
					out = append(out, issue("go.mod", "missing tool directive %s (go get -tool %s)", WatchTool, WatchTool))
				}
			}
			used := map[string]bool{}
			for _, c := range wnbConfigRE.FindAllString(raw, -1) {
				used[c] = true
				if !r.Has(c) {
					out = append(out, issue("Taskfile.yml", "a dev target uses %s, which does not exist", c))
				}
			}
			for _, c := range configs {
				// .wnb.yaml is watchnbuild's default, so it needs no mention.
				if c != ".wnb.yaml" && !used[c] {
					out = append(out, issue(c, "no dev target uses this config"))
				}
			}
			return out
		},
	},
	{
		ID:      "env.test-race",
		Summary: "The test target runs the race detector.",
		Check: func(r *repo.Repo) []Issue {
			tasks, raw, err := taskfile(r)
			if err != nil {
				return nil // env.taskfile-targets reports it.
			}
			node, ok := tasks["test"]
			if !ok {
				return nil
			}
			// Either inline, or through a variable (the starter's RACE, which
			// is empty on Windows, where the race detector needs cgo).
			body, _ := yaml.Marshal(&node)
			inline := strings.Contains(string(body), "-race")
			viaVar := strings.Contains(string(body), "{{.RACE}}") && strings.Contains(raw, "-race")
			if !inline && !viaVar {
				return []Issue{issue("Taskfile.yml", "the test target does not use -race")}
			}
			return nil
		},
	},
}

// taskfile parses Taskfile.yml and returns its tasks and raw text.
func taskfile(r *repo.Repo) (map[string]yaml.Node, string, error) {
	data, ok := r.Read("Taskfile.yml")
	if !ok {
		return nil, "", errMissingTaskfile
	}
	var tf struct {
		Tasks map[string]yaml.Node `yaml:"tasks"`
	}
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, "", err
	}
	return tf.Tasks, string(data), nil
}

var errMissingTaskfile = errors.New("missing Taskfile.yml")
