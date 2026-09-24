package rules

import (
	"bytes"
	"strings"

	"github.com/discobox-ai/repostd/internal/managed"
	"github.com/discobox-ai/repostd/internal/repo"
)

var agentsRules = []Rule{
	{
		ID:      "agents.skills-symlink",
		Summary: ".claude/skills is a symlink to ../.agents/skills.",
		Check: func(r *repo.Repo) []Issue {
			const p = ".claude/skills"
			target, link := r.Readlink(p)
			switch {
			case !r.Exists(p):
				return []Issue{issue(p, "missing symlink to ../.agents/skills (run repocheck sync)")}
			case !link:
				return []Issue{issue(p, "real directory; move its skills to .agents/skills and symlink it")}
			case target != "../.agents/skills":
				return []Issue{issue(p, "points at %q, not ../.agents/skills", target)}
			}
			return nil
		},
	},
	{
		ID:      "agents.no-settings-local",
		Summary: ".claude/settings.local.json is not committed.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, f := range r.Match("settings.local.json") {
				out = append(out, issue(f, "per-user settings are committed; untrack and ignore it"))
			}
			return out
		},
	},
}

var lintRules = []Rule{
	{
		ID:      "managed.files",
		Summary: "Managed files match repostd (.golangci.yml outside its local blocks).",
		Check: func(r *repo.Repo) []Issue {
			want, err := managed.Expected(r)
			if err != nil {
				return []Issue{issue("", "computing managed files: %v", err)}
			}
			var out []Issue
			for _, f := range want {
				if f.Path == managed.ADRReadme {
					continue // adr.index reports it.
				}
				if dropped := managed.Dropped(r); f.Path == managed.GolangciPath && len(dropped) > 0 {
					out = append(out, issue(f.Path, "local blocks %s are no longer in the standard, and sync will not drop their lines; move them into a remaining local block", strings.Join(dropped, ", ")))
					continue
				}
				got, ok := r.Read(f.Path)
				switch {
				case !ok:
					out = append(out, issue(f.Path, "missing managed file (run repocheck sync)"))
				case !bytes.Equal(got, f.Data):
					out = append(out, issue(f.Path, "differs from repostd (run repocheck sync; change it upstream, not here)"))
				}
			}
			return out
		},
	},
}
