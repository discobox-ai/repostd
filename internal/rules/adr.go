package rules

import (
	"bytes"
	"errors"
	"slices"

	"github.com/discobox-ai/repostd/internal/adr"
	"github.com/discobox-ai/repostd/internal/managed"
	"github.com/discobox-ai/repostd/internal/repo"
)

var adrRules = []Rule{
	{
		ID:      "adr.files",
		Summary: "docs/adr has README.md with index markers and template.md.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			readme, ok := r.Read(managed.ADRReadme)
			switch {
			case !ok:
				out = append(out, issue(managed.ADRReadme, "missing ADR README (run repocheck init)"))
			case !bytes.Contains(readme, []byte(adr.IndexBegin)) || !bytes.Contains(readme, []byte(adr.IndexEnd)):
				out = append(out, issue(managed.ADRReadme, "missing index markers %s and %s", adr.IndexBegin, adr.IndexEnd))
			}
			return out
		},
	},
	{
		ID:      "adr.format",
		Summary: "ADR filename, header, status, date, and sections follow the template.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, a := range managed.ADRs(r) {
				for _, p := range a.Problems {
					out = append(out, issue(a.Path, "%s", p))
				}
			}
			return out
		},
	},
	{
		ID:      "adr.unique",
		Summary: "ADR IDs are unique.",
		Check: func(r *repo.Repo) []Issue {
			byID := map[string][]string{}
			for _, a := range managed.ADRs(r) {
				if a.ID != "" {
					byID[a.ID] = append(byID[a.ID], a.Path)
				}
			}
			var out []Issue
			for _, a := range managed.ADRs(r) {
				if paths := byID[a.ID]; len(paths) > 1 && paths[0] != a.Path {
					out = append(out, issue(a.Path, "ADR ID %s is also used by %s", a.ID, paths[0]))
				}
			}
			return out
		},
	},
	{
		ID:      "adr.supersede",
		Summary: "\"Superseded by B\" and \"Supersedes: A\" always appear together.",
		Check: func(r *repo.Repo) []Issue {
			adrs := managed.ADRs(r)
			byID := map[string]*adr.ADR{}
			for _, a := range adrs {
				if _, dup := byID[a.ID]; !dup {
					byID[a.ID] = a
				}
			}
			var out []Issue
			for _, a := range adrs {
				if a.SupersededBy != "" {
					b, ok := byID[a.SupersededBy]
					switch {
					case !ok:
						out = append(out, issue(a.Path, "superseded by %s, which does not exist", a.SupersededBy))
					case !slices.Contains(b.Supersedes, a.ID):
						out = append(out, issue(b.Path, "%s says it is superseded by %s, but this ADR has no \"- **Supersedes**: %s\"", a.ID, b.ID, a.ID))
					}
				}
				for _, n := range a.Supersedes {
					old, ok := byID[n]
					switch {
					case !ok:
						out = append(out, issue(a.Path, "supersedes %s, which does not exist", n))
					case old.SupersededBy != a.ID:
						out = append(out, issue(old.Path, "%s supersedes this ADR, but its status is %q, not \"Superseded by %s\"", a.ID, old.Status, a.ID))
					}
				}
			}
			return out
		},
	},
	{
		ID:      "adr.index",
		Summary: "The ADR index in docs/adr/README.md is current.",
		Check: func(r *repo.Repo) []Issue {
			readme, ok := r.Read(managed.ADRReadme)
			if !ok {
				return nil
			}
			want, err := adr.ReplaceIndex(readme, adr.Index(managed.ADRs(r)))
			if errors.Is(err, adr.ErrNoIndexMarkers) {
				return nil // adr.files reports it.
			}
			if err != nil || !bytes.Equal(want, readme) {
				return []Issue{issue(managed.ADRReadme, "ADR index is stale (run repocheck sync)")}
			}
			return nil
		},
	},
}
