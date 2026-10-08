package rules

import (
	"strings"

	"github.com/discobox-ai/repostd/internal/repo"
)

// ignoredPaths are the patterns .gitignore covers on each stack, normalized
// by normalizeIgnore.
var ignoredPaths = map[repo.Stack][]string{
	repo.Go:   {"build", ".direnv", ".env", "result"},
	repo.Node: {"node_modules", ".env"},
}

var gitRules = []Rule{
	{
		ID:      "git.attributes",
		Summary: ".gitattributes starts with `* text=auto eol=lf`.",
		Check: func(r *repo.Repo) []Issue {
			data, ok := r.Read(".gitattributes")
			if !ok {
				return []Issue{issue(".gitattributes", "missing .gitattributes")}
			}
			for l := range strings.Lines(string(data)) {
				t := strings.TrimSpace(l)
				if t == "" || strings.HasPrefix(t, "#") {
					continue
				}
				if strings.Join(strings.Fields(t), " ") == "* text=auto eol=lf" {
					return nil
				}
				break
			}
			return []Issue{issue(".gitattributes", "first rule is not `* text=auto eol=lf`")}
		},
	},
	{
		ID:      "git.ignore",
		Summary: ".gitignore covers build/, .direnv/, .env, and result; a node repo's covers node_modules/ and .env.",
		Check: func(r *repo.Repo) []Issue {
			data, ok := r.Read(".gitignore")
			if !ok {
				return []Issue{issue(".gitignore", "missing .gitignore")}
			}
			have := map[string]bool{}
			for l := range strings.Lines(string(data)) {
				have[normalizeIgnore(l)] = true
			}
			var out []Issue
			for _, p := range ignoredPaths[r.Stack] {
				if !have[p] {
					out = append(out, issue(".gitignore", "does not ignore %s", p))
				}
			}
			return out
		},
	},
}

// normalizeIgnore reduces "/build/", "build", ".env*", and "/result*" to
// "build", ".env", and "result".
func normalizeIgnore(l string) string {
	t := strings.TrimSpace(l)
	t = strings.Trim(t, "/")
	return strings.TrimSuffix(t, "*")
}
