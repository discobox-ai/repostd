package rules

import (
	"path"
	"sort"
	"strings"

	"github.com/discobox-ai/repostd/internal/repo"
)

// RootDirs are the only non-dot directories allowed at a repo root.
var RootDirs = []string{"cmd", "docs", "internal", "pkg", "scripts", "test"}

var layoutRules = []Rule{
	{
		ID:      "layout.root-dirs",
		Summary: "Root directories are only " + strings.Join(RootDirs, ", ") + ", plus dot-directories.",
		Check: func(r *repo.Repo) []Issue {
			allowed := map[string]bool{}
			for _, d := range RootDirs {
				allowed[d] = true
			}
			seen := map[string]bool{}
			for _, f := range r.Files {
				dir, _, ok := strings.Cut(f, "/")
				if !ok || strings.HasPrefix(dir, ".") || allowed[dir] {
					continue
				}
				seen[dir] = true
			}
			dirs := make([]string, 0, len(seen))
			for d := range seen {
				dirs = append(dirs, d)
			}
			sort.Strings(dirs)
			var out []Issue
			for _, d := range dirs {
				out = append(out, issue(d, "top-level directory %q is not one of %s", d, strings.Join(RootDirs, ", ")))
			}
			return out
		},
	},
	{
		ID:      "layout.root-go",
		Summary: "No Go package at the repo root.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, f := range r.Files {
				if !strings.Contains(f, "/") && path.Ext(f) == ".go" {
					out = append(out, issue(f, "Go file at the repo root; move the package under cmd/, pkg/, or internal/"))
				}
			}
			return out
		},
	},
	{
		ID:      "layout.go-work",
		Summary: "One Go module; go.work needs a waiver.",
		Check: func(r *repo.Repo) []Issue {
			if r.Has("go.work") {
				return []Issue{issue("go.work", "multi-module workspace; use one module or waive with the reason")}
			}
			return nil
		},
	},
	{
		ID:      "layout.no-makefile",
		Summary: "No Makefile; targets live in Taskfile.yml.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, name := range []string{"Makefile", "GNUmakefile", "makefile"} {
				for _, f := range r.Match(name) {
					out = append(out, issue(f, "Makefile; move targets to Taskfile.yml"))
				}
			}
			return out
		},
	},
	{
		ID:      "layout.no-tools-go",
		Summary: "No tools.go; pin tools with go.mod tool directives.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, f := range r.Match("*.go") {
				data, _ := r.Read(f)
				for l := range strings.Lines(string(data)) {
					if c, ok := strings.CutPrefix(strings.TrimSpace(l), "//go:build "); ok && strings.TrimSpace(c) == "tools" {
						out = append(out, issue(f, "tools build-tag file; use `go get -tool` instead"))
						break
					}
				}
			}
			return out
		},
	},
	{
		ID:      "layout.no-goreleaser",
		Summary: "No goreleaser; release logic is Taskfile targets backed by Go programs.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, pat := range []string{".goreleaser.yml", ".goreleaser.yaml"} {
				for _, f := range r.Match(pat) {
					out = append(out, issue(f, "goreleaser config; release through task release:* targets"))
				}
			}
			return out
		},
	},
}
