package rules

import (
	"path"
	"regexp"
	"strings"

	"github.com/discobox-ai/repostd/internal/repo"
)

// Where a discobox declares what runs in a development sandbox.
const (
	HooksDir    = ".discobox/hooks"
	ServicesDir = ".discobox/services"
)

// directTooling matches a build, test or lint tool run straight from a hook,
// which is the drift the Taskfile, or a node repo's scripts, exist to prevent.
var directTooling = regexp.MustCompile(`(?m)^\s*(?:go (?:build|test|generate|vet|run)\b|gofmt\b|golangci-lint\b|shellcheck\b|actionlint\b|(?:npx|pnpm exec|pnpm dlx)\s|oxlint\b|oxfmt\b|eslint\b|prettier\b|tsc\b|vitest\b|jest\b|node --test\b|remix (?:test|doctor)\b)`)

// frontmatter is the "#---" header a hook or service declares itself with.
var frontmatterRE = regexp.MustCompile(`(?m)^#---\s*$`)

// heredocRE matches the start of a heredoc and captures its terminator.
var heredocRE = regexp.MustCompile(`<<-?\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

var discoboxRules = []Rule{
	{
		ID:      "discobox.hooks",
		Summary: "Hooks declare themselves and invoke Taskfile targets or package.json scripts, never tooling directly.",
		Check: func(r *repo.Repo) []Issue {
			hooks := scripts(r, HooksDir)
			if len(hooks) == 0 {
				return []Issue{issue(HooksDir, "no hooks: a discobox runs them on change, and they are what keeps local work and CI the same (run repocheck init)")}
			}
			var out []Issue
			for _, f := range hooks {
				data, _ := r.Read(f)
				body := string(data)
				if !declares(body) {
					out = append(out, issue(f, "no `#---` frontmatter declaring at least `name:`"))
				}
				if m := directTooling.FindString(stripHeredocs(body)); m != "" {
					out = append(out, issue(f, "runs %q directly; a hook is a thin trigger for `%s`", strings.TrimSpace(m), runTarget(r.Stack, "<target>")))
				}
			}
			return out
		},
	},
	{
		ID:      "discobox.dev-service",
		Summary: "A discobox service starts the dev target, or a node repo's dev script where there is one.",
		Check: func(r *repo.Repo) []Issue {
			if !hasDev(r) {
				return nil
			}
			run := runTarget(r.Stack, "dev")
			for _, f := range scripts(r, ServicesDir) {
				data, _ := r.Read(f)
				if strings.Contains(string(data), run) {
					return nil
				}
			}
			return []Issue{issue(ServicesDir, "no service runs `%s`, so the dev loop does not start in a discobox", run)}
		},
	},
	{
		ID:      "discobox.services",
		Summary: "Services declare themselves in frontmatter.",
		Check: func(r *repo.Repo) []Issue {
			// What a service runs is not constrained: an auxiliary process
			// such as a metrics dashboard is a container, not a task.
			var out []Issue
			for _, f := range scripts(r, ServicesDir) {
				data, _ := r.Read(f)
				if !declares(string(data)) {
					out = append(out, issue(f, "no `#---` frontmatter declaring at least `name:`"))
				}
			}
			return out
		},
	},
}

// hasDev reports whether the repo has a dev target (Go) or script (node).
func hasDev(r *repo.Repo) bool {
	if r.Stack == repo.Node {
		_, ok := r.Package.Scripts["dev"]
		return ok
	}
	tasks, _, err := taskfile(r)
	if err != nil {
		return false // env.taskfile-targets reports it.
	}
	_, ok := tasks["dev"]
	return ok
}

// runTarget is how a hook, service or CI step runs target on stack.
func runTarget(stack repo.Stack, target string) string {
	if stack == repo.Node {
		return "pnpm run " + target
	}
	return "go tool task " + target
}

// targetHome is where a stack's targets are defined.
func targetHome(stack repo.Stack) string {
	if stack == repo.Node {
		return "package.json scripts"
	}
	return "Taskfile.yml"
}

// declares reports whether a script has the "#---" frontmatter block.
func declares(body string) bool {
	return len(frontmatterRE.FindAllString(body, -1)) >= 2 && strings.Contains(body, "# name:")
}

// stripHeredocs removes heredoc bodies, so that a tool named in an error
// message a hook prints is not read as the hook running it.
func stripHeredocs(body string) string {
	var out strings.Builder
	var terminator string
	for line := range strings.Lines(body) {
		if terminator != "" {
			if strings.TrimSpace(line) == terminator {
				terminator = ""
			}
			continue
		}
		if m := heredocRE.FindStringSubmatch(line); m != nil {
			terminator = m[1]
		}
		out.WriteString(line)
	}
	return out.String()
}

// scripts returns the shell scripts directly inside dir.
func scripts(r *repo.Repo, dir string) []string {
	var out []string
	for _, f := range r.Under(dir) {
		if path.Dir(f) == dir && path.Ext(f) == ".sh" {
			out = append(out, f)
		}
	}
	return out
}
