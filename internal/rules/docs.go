package rules

import (
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/discobox-ai/repostd/internal/repo"
)

// AgentsSections are the "##" sections the root AGENTS.md has.
var AgentsSections = []string{
	"Project Structure",
	"Shared Libraries",
	"Git Workflow",
	"Commands",
	"Implementation Quality",
	"Package Design Docs",
	"Architecture Decision Records",
}

// DESIGN.md length limits, in lines.
const (
	DesignWarnLines  = 300
	DesignErrorLines = 600
)

var requiredDocs = []string{"README.md", "DESIGN.md", "LICENSE", "NOTICE", "SECURITY.md"}

var mermaidTypes = []string{
	"architecture-beta", "block-beta", "C4Component", "C4Container", "C4Context",
	"C4Deployment", "C4Dynamic", "classDiagram", "erDiagram", "flowchart", "gantt",
	"gitGraph", "graph", "journey", "kanban", "mindmap", "packet-beta", "pie",
	"quadrantChart", "radar-beta", "requirementDiagram", "sankey-beta",
	"sequenceDiagram", "stateDiagram", "stateDiagram-v2", "timeline",
	"treemap-beta", "xychart-beta", "zenuml",
}

var docsRules = []Rule{
	{
		ID:      "docs.agents-symlink",
		Summary: "AGENTS.md is a regular file; CLAUDE.md beside it is a symlink to AGENTS.md.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			if !r.Has("AGENTS.md") {
				out = append(out, issue("AGENTS.md", "missing root AGENTS.md"))
			}
			dirs := map[string]bool{}
			for _, f := range append(r.Match("AGENTS.md"), r.Match("CLAUDE.md")...) {
				dirs[path.Dir(f)] = true
			}
			for _, dir := range sortedKeys(dirs) {
				agents, claude := path.Join(dir, "AGENTS.md"), path.Join(dir, "CLAUDE.md")
				if _, link := r.Readlink(agents); link {
					out = append(out, issue(agents, "AGENTS.md is a symlink; it must be the real file"))
				} else if !r.Exists(agents) {
					out = append(out, issue(agents, "CLAUDE.md without an AGENTS.md; rename it to AGENTS.md"))
				}
				target, link := r.Readlink(claude)
				switch {
				case !r.Exists(claude):
					out = append(out, issue(claude, "missing CLAUDE.md symlink to AGENTS.md (run repocheck sync)"))
				case !link:
					out = append(out, issue(claude, "CLAUDE.md is a regular file; make it a symlink to AGENTS.md"))
				case target != "AGENTS.md":
					out = append(out, issue(claude, "CLAUDE.md points at %q, not AGENTS.md", target))
				}
			}
			return out
		},
	},
	{
		ID:      "docs.agents-sections",
		Summary: "The root AGENTS.md has the standard sections.",
		Check: func(r *repo.Repo) []Issue {
			data, ok := r.Read("AGENTS.md")
			if !ok {
				return nil
			}
			have := map[string]bool{}
			for _, h := range headings(string(data), "## ") {
				have[h] = true
			}
			var out []Issue
			for _, s := range AgentsSections {
				if !have[s] {
					out = append(out, issue("AGENTS.md", "missing section \"## %s\"", s))
				}
			}
			return out
		},
	},
	{
		ID:      "docs.required-files",
		Summary: "README.md, DESIGN.md, LICENSE, NOTICE, and SECURITY.md exist.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, f := range requiredDocs {
				if !r.Has(f) {
					out = append(out, issue(f, "missing %s", f))
				}
			}
			return out
		},
	},
	{
		ID:      "docs.license",
		Summary: "LICENSE is Apache-2.0.",
		Check: func(r *repo.Repo) []Issue {
			data, ok := r.Read("LICENSE")
			if !ok {
				return nil
			}
			s := string(data)
			if !strings.Contains(s, "Apache License") || !strings.Contains(s, "Version 2.0") {
				return []Issue{issue("LICENSE", "LICENSE is not the Apache License, Version 2.0")}
			}
			return nil
		},
	},
	{
		ID:      "docs.review-design",
		Summary: "A REVIEW.md has a DESIGN.md beside it.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, f := range r.Match("REVIEW.md") {
				if !r.Has(path.Join(path.Dir(f), "DESIGN.md")) {
					out = append(out, issue(f, "REVIEW.md without a DESIGN.md beside it"))
				}
			}
			return out
		},
	},
	{
		ID:      "docs.design-length",
		Summary: "DESIGN.md warns over 300 lines and fails over 600.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, f := range r.Match("DESIGN.md") {
				data, _ := r.Read(f)
				n := strings.Count(string(data), "\n")
				switch {
				case n > DesignErrorLines:
					out = append(out, issue(f, "%d lines (limit %d); split detail into child packages' DESIGN.md", n, DesignErrorLines))
				case n > DesignWarnLines:
					out = append(out, warning(f, "%d lines (warning at %d); consider splitting into child packages' DESIGN.md", n, DesignWarnLines))
				}
			}
			return out
		},
	},
	{
		ID:      "docs.no-plans",
		Summary: "No plan or task documents in the repo.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, dir := range []string{"docs/tasks", "docs/plans"} {
				if len(r.Under(dir)) > 0 {
					out = append(out, issue(dir, "plans and task notes belong in the branch or task, not the repo"))
				}
			}
			return out
		},
	},
	{
		ID:      "docs.mermaid",
		Summary: "Every Mermaid block is closed and starts with a known diagram type.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, f := range r.Match("*.md") {
				data, _ := r.Read(f)
				out = append(out, mermaidIssues(f, string(data))...)
			}
			return out
		},
	},
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// headings returns the text of lines starting with prefix, outside fences.
func headings(s, prefix string) []string {
	var out []string
	inFence := false
	for l := range strings.Lines(s) {
		l = strings.TrimRight(l, "\r\n")
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inFence = !inFence
			continue
		}
		if !inFence && strings.HasPrefix(l, prefix) {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(l, prefix)))
		}
	}
	return out
}

func mermaidIssues(f, s string) []Issue {
	var out []Issue
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		fence := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(fence, "```") {
			continue
		}
		isMermaid := strings.TrimSpace(strings.TrimPrefix(fence, "```")) == "mermaid"
		start := i
		var body []string
		for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
			body = append(body, lines[i])
		}
		if !isMermaid {
			continue
		}
		if i == len(lines) {
			out = append(out, issue(f, "line %d: unclosed mermaid block", start+1))
			continue
		}
		if kind := mermaidKind(body); !knownMermaid(kind) {
			out = append(out, issue(f, "line %d: mermaid block does not start with a known diagram type (got %q)", start+1, kind))
		}
	}
	return out
}

// mermaidKind returns the diagram keyword, skipping front matter, comments,
// and init directives.
func mermaidKind(body []string) string {
	inFront := false
	for i, l := range body {
		t := strings.TrimSpace(l)
		if t == "---" && (i == 0 || inFront) {
			inFront = !inFront
			continue
		}
		if inFront || t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		kind, _, _ := strings.Cut(t, " ")
		return strings.TrimSuffix(kind, ";")
	}
	return ""
}

func knownMermaid(kind string) bool {
	return slices.Contains(mermaidTypes, kind)
}
