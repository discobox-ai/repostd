package rules

import (
	"regexp"
	"strings"

	"github.com/discobox-ai/repostd/internal/repo"
)

const testify = "github.com/stretchr/testify"

var (
	testTagRE = regexp.MustCompile(`\b(integration|e2e)\b`)
	testEnvRE = regexp.MustCompile(`\b[A-Z][A-Z0-9_]*_(?:INTEGRATION|E2E)\b`)
	// envReadRE matches a variable name a test reads: passed to
	// os.Getenv/LookupEnv directly, or declared as a string constant (the
	// `const dockerIntegrationEnv = "..."` pattern). A name that only appears
	// as data, such as a container's env, is not a read.
	envReadRE = regexp.MustCompile(`(?:Getenv|LookupEnv)\("([A-Z][A-Z0-9_]*)"\)|(?m)^\s*(?:const\s+)?[A-Za-z_][A-Za-z0-9_]*\s*=\s*"([A-Z][A-Z0-9_]*)"`)
)

var testsRules = []Rule{
	{
		ID:      "tests.no-testify",
		Summary: "Tests use the standard testing package; testify is banned.",
		Check: func(r *repo.Repo) []Issue {
			if r.Mod == nil {
				return nil
			}
			for _, req := range r.Mod.Require {
				if !req.Indirect && strings.HasPrefix(req.Mod.Path, testify) {
					return []Issue{issue("go.mod", "requires %s; use the standard testing package", req.Mod.Path)}
				}
			}
			return nil
		},
	},
	{
		ID:      "tests.no-integration-tags",
		Summary: "Integration and e2e tests skip on an env var instead of using build tags.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, f := range r.Match("*_test.go") {
				data, _ := r.Read(f)
				for l := range strings.Lines(string(data)) {
					c, ok := strings.CutPrefix(strings.TrimSpace(l), "//go:build ")
					if ok && testTagRE.MatchString(c) {
						out = append(out, issue(f, "build tag %q; name the file *_integration_test.go and t.Skip unless the env var is set", strings.TrimSpace(c)))
						break
					}
				}
			}
			return out
		},
	},
	{
		ID:      "tests.env-read",
		Summary: "Every *_INTEGRATION or *_E2E variable the Taskfile sets is read by a test.",
		Check: func(r *repo.Repo) []Issue {
			data, ok := r.Read("Taskfile.yml")
			if !ok {
				return nil
			}
			read := map[string]bool{}
			for _, f := range r.Match("*_test.go") {
				d, _ := r.Read(f)
				for _, m := range envReadRE.FindAllStringSubmatch(string(d), -1) {
					read[m[1]+m[2]] = true
				}
			}
			seen := map[string]bool{}
			var out []Issue
			for _, name := range testEnvRE.FindAllString(string(data), -1) {
				if seen[name] {
					continue
				}
				seen[name] = true
				if !read[name] {
					out = append(out, issue("Taskfile.yml", "sets %s, but no test reads it, so those tests silently skip", name))
				}
			}
			return out
		},
	},
}
