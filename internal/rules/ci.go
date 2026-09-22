package rules

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/discobox-ai/repostd/internal/repo"
)

const (
	ciWorkflow      = ".github/workflows/ci.yml"
	releaseWorkflow = ".github/workflows/release.yml"
	taskAction      = "./.github/actions/task"
	// RenovatePreset is the shared Renovate preset every repo extends.
	RenovatePreset = "github>discobox-ai/repostd"
)

// CIJobs are the jobs ci.yml has, each running the same-named target.
var CIJobs = []string{"verify", "check", "test"}

var (
	usesRE      = regexp.MustCompile(`^\s*(?:-\s+)?uses:\s*["']?([^\s"'#]+)["']?\s*(?:#\s*(\S+))?`)
	pinnedRE    = regexp.MustCompile(`@[0-9a-f]{40}$`)
	versionRE   = regexp.MustCompile(`^v\d+(\.\d+)*$`)
	runStepRE   = regexp.MustCompile(`^(nix develop -c )?go tool task [A-Za-z0-9:_.-]+( .*)?$`)
	matrixRefRE = regexp.MustCompile(`^\$\{\{\s*matrix\.([A-Za-z0-9_-]+)\s*\}\}$`)
)

type workflow struct {
	On          yaml.Node      `yaml:"on"`
	Permissions yaml.Node      `yaml:"permissions"`
	Jobs        map[string]job `yaml:"jobs"`
}

type job struct {
	RunsOn   yaml.Node `yaml:"runs-on"`
	Uses     string    `yaml:"uses"`
	Strategy struct {
		Matrix map[string]yaml.Node `yaml:"matrix"`
	} `yaml:"strategy"`
	Steps []step `yaml:"steps"`
}

type step struct {
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
}

// action returns the step's action without its ref.
func (s step) action() string {
	a, _, _ := strings.Cut(s.Uses, "@")
	return a
}

var ciRules = []Rule{
	{
		ID:      "ci.workflow",
		Summary: "ci.yml runs verify, check, and test through the task action on PRs and pushes to main.",
		Check: func(r *repo.Repo) []Issue {
			w, err := loadWorkflow(r, ciWorkflow)
			if err != nil {
				return []Issue{issue(ciWorkflow, "%v", err)}
			}
			var out []Issue
			triggers := mapKeys(&w.On)
			if !slices.Contains(triggers, "pull_request") {
				out = append(out, issue(ciWorkflow, "does not run on pull_request"))
			}
			if !pushesMain(&w.On) {
				out = append(out, issue(ciWorkflow, "does not run on push to main"))
			}
			for _, name := range CIJobs {
				j, ok := w.Jobs[name]
				if !ok {
					out = append(out, issue(ciWorkflow, "missing job %q", name))
					continue
				}
				if !runsTarget(j, name) {
					out = append(out, issue(ciWorkflow, "job %q does not run %s with target %q", name, taskAction, name))
				}
			}
			return out
		},
	},
	{
		ID:      "ci.runners",
		Summary: "Every job runs on a Depot runner.",
		Check: func(r *repo.Repo) []Issue {
			return eachJob(r, func(p, name string, j job) []Issue {
				if j.Uses != "" {
					return nil // A reusable workflow picks its own runners.
				}
				labels, err := runnerLabels(j)
				if err != nil {
					return []Issue{issue(p, "job %q: %v", name, err)}
				}
				var out []Issue
				for _, l := range labels {
					if !strings.HasPrefix(l, "depot-") {
						out = append(out, issue(p, "job %q runs on %q, not a depot-* runner", name, l))
					}
				}
				return out
			})
		},
	},
	{
		ID:      "ci.no-setup",
		Summary: "No actions/setup-*; only a depot-windows job may use actions/setup-go with go-version-file: go.mod.",
		Check: func(r *repo.Repo) []Issue {
			return eachJob(r, func(p, name string, j job) []Issue {
				labels, _ := runnerLabels(j)
				windows := len(labels) > 0
				for _, l := range labels {
					windows = windows && strings.HasPrefix(l, "depot-windows")
				}
				var out []Issue
				for _, s := range j.Steps {
					a := s.action()
					if !strings.HasPrefix(a, "actions/setup-") {
						continue
					}
					switch {
					case !windows:
						out = append(out, issue(p, "job %q uses %s; the environment comes from the flake", name, a))
					case a != "actions/setup-go":
						out = append(out, issue(p, "job %q uses %s; Windows jobs may use actions/setup-go only", name, a))
					case fmt.Sprint(s.With["go-version-file"]) != "go.mod":
						out = append(out, issue(p, "job %q: actions/setup-go must use go-version-file: go.mod", name))
					}
				}
				return out
			})
		},
	},
	{
		ID:      "ci.run-steps",
		Summary: "Every workflow run: step is one `[nix develop -c] go tool task <target>`.",
		Check: func(r *repo.Repo) []Issue {
			return eachJob(r, func(p, name string, j job) []Issue {
				var out []Issue
				for _, s := range j.Steps {
					if s.Run == "" {
						continue
					}
					if !runStepRE.MatchString(strings.TrimSpace(s.Run)) {
						out = append(out, issue(p, "job %q: run step %q is not a single task invocation; move the logic into Taskfile.yml", name, firstLine(s.Run)))
					}
				}
				return out
			})
		},
	},
	{
		ID:      "ci.release",
		Summary: "release.yml, where present, runs on v* tags and calls only release:* targets.",
		Check: func(r *repo.Repo) []Issue {
			if !r.Has(releaseWorkflow) {
				return nil
			}
			w, err := loadWorkflow(r, releaseWorkflow)
			if err != nil {
				return []Issue{issue(releaseWorkflow, "%v", err)}
			}
			var out []Issue
			if !pushesTag(&w.On) {
				out = append(out, issue(releaseWorkflow, "does not run on push of v* tags"))
			}
			for _, name := range sortedJobs(w) {
				for _, s := range w.Jobs[name].Steps {
					if target := taskTarget(s); target != "" && !strings.HasPrefix(target, "release:") {
						out = append(out, issue(releaseWorkflow, "job %q runs %q; release runs release:* targets only", name, target))
					}
				}
			}
			return out
		},
	},
	{
		ID:      "ci.pinned",
		Summary: "Every uses: is pinned to a 40-character commit hash with a # vX.Y.Z comment.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, p := range actionFiles(r) {
				data, _ := r.Read(p)
				for i, l := range strings.Split(string(data), "\n") {
					m := usesRE.FindStringSubmatch(l)
					if m == nil || strings.HasPrefix(m[1], "./") {
						continue
					}
					if strings.HasPrefix(m[1], "docker://") {
						if !strings.Contains(m[1], "@sha256:") {
							out = append(out, issue(p, "line %d: %s is not pinned by digest", i+1, m[1]))
						}
						continue
					}
					if !pinnedRE.MatchString(m[1]) {
						out = append(out, issue(p, "line %d: %s is not pinned to a commit hash", i+1, m[1]))
					} else if !versionRE.MatchString(m[2]) {
						out = append(out, issue(p, "line %d: %s has no `# vX.Y.Z` comment", i+1, m[1]))
					}
				}
			}
			return out
		},
	},
	{
		ID:      "ci.permissions",
		Summary: "Every workflow sets top-level permissions that are read-only.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, p := range workflowFiles(r) {
				w, err := loadWorkflow(r, p)
				if err != nil {
					out = append(out, issue(p, "%v", err))
					continue
				}
				perm := &w.Permissions
				switch {
				case perm.Kind == 0:
					out = append(out, issue(p, "no top-level permissions; set `permissions: contents: read`"))
				case perm.Kind == yaml.MappingNode:
					for i := 0; i+1 < len(perm.Content); i += 2 {
						if v := perm.Content[i+1].Value; v != "read" && v != "none" {
							out = append(out, issue(p, "top-level permission %s: %s; widen permissions per job instead", perm.Content[i].Value, v))
						}
					}
				case perm.Value != "read-all":
					out = append(out, issue(p, "top-level permissions %q; set `permissions: contents: read`", perm.Value))
				}
			}
			return out
		},
	},
	{
		ID:      "ci.checkout-credentials",
		Summary: "actions/checkout sets persist-credentials: false.",
		Check: func(r *repo.Repo) []Issue {
			return eachJob(r, func(p, name string, j job) []Issue {
				var out []Issue
				for _, s := range j.Steps {
					if s.action() == "actions/checkout" && fmt.Sprint(s.With["persist-credentials"]) != "false" {
						out = append(out, issue(p, "job %q: actions/checkout without persist-credentials: false", name))
					}
				}
				return out
			})
		},
	},
	{
		ID:      "ci.renovate",
		Summary: "renovate.json extends " + RenovatePreset + ".",
		Check: func(r *repo.Repo) []Issue {
			data, ok := r.Read("renovate.json")
			if !ok {
				return []Issue{issue("renovate.json", "missing renovate.json")}
			}
			var cfg struct {
				Extends []string `json:"extends"`
			}
			if err := json.Unmarshal(data, &cfg); err != nil {
				return []Issue{issue("renovate.json", "%v", err)}
			}
			if !slices.Contains(cfg.Extends, RenovatePreset) {
				return []Issue{issue("renovate.json", "does not extend %q", RenovatePreset)}
			}
			return nil
		},
	},
}

func workflowFiles(r *repo.Repo) []string {
	var out []string
	for _, f := range r.Under(".github/workflows") {
		if path.Dir(f) == ".github/workflows" && (path.Ext(f) == ".yml" || path.Ext(f) == ".yaml") {
			out = append(out, f)
		}
	}
	return out
}

// actionFiles are the workflows and local composite actions.
func actionFiles(r *repo.Repo) []string {
	out := workflowFiles(r)
	for _, f := range r.Under(".github/actions") {
		if b := path.Base(f); b == "action.yml" || b == "action.yaml" {
			out = append(out, f)
		}
	}
	return out
}

func loadWorkflow(r *repo.Repo, p string) (*workflow, error) {
	data, ok := r.Read(p)
	if !ok {
		return nil, fmt.Errorf("missing %s", p)
	}
	var w workflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

// eachJob runs fn over every job of every workflow, in a stable order.
func eachJob(r *repo.Repo, fn func(p, name string, j job) []Issue) []Issue {
	var out []Issue
	for _, p := range workflowFiles(r) {
		w, err := loadWorkflow(r, p)
		if err != nil {
			continue // ci.permissions reports unparsable workflows.
		}
		for _, name := range sortedJobs(w) {
			out = append(out, fn(p, name, w.Jobs[name])...)
		}
	}
	return out
}

func sortedJobs(w *workflow) []string {
	names := make([]string, 0, len(w.Jobs))
	for n := range w.Jobs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// runnerLabels resolves a job's runs-on, including a single matrix reference.
func runnerLabels(j job) ([]string, error) {
	n := &j.RunsOn
	switch n.Kind {
	case 0:
		return nil, nil
	case yaml.ScalarNode:
		m := matrixRefRE.FindStringSubmatch(n.Value)
		if m == nil {
			if strings.Contains(n.Value, "${{") {
				return nil, fmt.Errorf("runs-on %q cannot be checked; use a literal or ${{ matrix.<key> }}", n.Value)
			}
			return []string{n.Value}, nil
		}
		if values, ok := j.Strategy.Matrix[m[1]]; ok {
			return scalars(&values), nil
		}
		// matrix.include entries: a list of mappings, each setting the key.
		var labels []string
		if include, ok := j.Strategy.Matrix["include"]; ok {
			for _, entry := range include.Content {
				if v, ok := mapValue(entry, m[1]); ok {
					labels = append(labels, scalars(v)...)
				}
			}
		}
		if len(labels) == 0 {
			return nil, fmt.Errorf("runs-on refers to matrix.%s, which has no literal values", m[1])
		}
		return labels, nil
	case yaml.SequenceNode:
		return scalars(n), nil
	default:
		return nil, fmt.Errorf("runs-on groups cannot be checked; use a depot-* label")
	}
}

func scalars(n *yaml.Node) []string {
	if n.Kind == yaml.ScalarNode {
		return []string{n.Value}
	}
	var out []string
	for _, c := range n.Content {
		if c.Kind == yaml.ScalarNode {
			out = append(out, c.Value)
		}
	}
	return out
}

// mapKeys returns the keys of a mapping node, or the value(s) of a scalar or
// sequence node (the short forms of `on:`).
func mapKeys(n *yaml.Node) []string {
	if n.Kind != yaml.MappingNode {
		return scalars(n)
	}
	var out []string
	for i := 0; i < len(n.Content); i += 2 {
		out = append(out, n.Content[i].Value)
	}
	return out
}

func mapValue(n *yaml.Node, key string) (*yaml.Node, bool) {
	if n.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1], true
		}
	}
	return nil, false
}

func pushesMain(on *yaml.Node) bool {
	push, ok := mapValue(on, "push")
	if !ok {
		return false
	}
	branches, ok := mapValue(push, "branches")
	return ok && slices.Contains(scalars(branches), "main")
}

func pushesTag(on *yaml.Node) bool {
	push, ok := mapValue(on, "push")
	if !ok {
		return false
	}
	tags, ok := mapValue(push, "tags")
	if !ok {
		return false
	}
	for _, t := range scalars(tags) {
		if strings.HasPrefix(t, "v") {
			return true
		}
	}
	return false
}

// runsTarget reports whether the job runs target through the task action.
func runsTarget(j job, target string) bool {
	for _, s := range j.Steps {
		if taskTarget(s) == target {
			return true
		}
	}
	return false
}

// taskTarget returns the Taskfile target a step runs, via the task action or
// a run: step, or "".
func taskTarget(s step) string {
	if s.Uses == taskAction {
		return fmt.Sprint(s.With["target"])
	}
	if m := runStepRE.FindStringSubmatch(strings.TrimSpace(s.Run)); m != nil {
		fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(s.Run), "nix develop -c "))
		return fields[3]
	}
	return ""
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return l
}
