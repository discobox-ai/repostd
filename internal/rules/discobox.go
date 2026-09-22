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
// which is the drift the Taskfile exists to prevent.
var directTooling = regexp.MustCompile(`(?m)^\s*(?:go (?:build|test|generate|vet|run)\b|gofmt\b|golangci-lint\b|shellcheck\b|actionlint\b)`)

// frontmatter is the "#---" header a hook or service declares itself with.
var frontmatterRE = regexp.MustCompile(`(?m)^#---\s*$`)

// heredocRE matches the start of a heredoc and captures its terminator.
var heredocRE = regexp.MustCompile(`<<-?\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

var discoboxRules = []Rule{
	{
		ID:      "discobox.hooks",
		Summary: "Hooks declare themselves and invoke Taskfile targets, never tooling directly.",
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
					out = append(out, issue(f, "runs %q directly; a hook is a thin trigger for `go tool task <target>`", strings.TrimSpace(m)))
				}
			}
			return out
		},
	},
	{
		ID:      "discobox.dev-service",
		Summary: "Where there is a dev target, a discobox service starts it.",
		Check: func(r *repo.Repo) []Issue {
			tasks, _, err := taskfile(r)
			if err != nil {
				return nil // env.taskfile-targets reports it.
			}
			if _, ok := tasks["dev"]; !ok {
				return nil
			}
			for _, f := range scripts(r, ServicesDir) {
				data, _ := r.Read(f)
				if strings.Contains(string(data), "go tool task dev") {
					return nil
				}
			}
			return []Issue{issue(ServicesDir, "no service runs `go tool task dev`, so the dev loop does not start in a discobox")}
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
