package canon_test

import (
	"slices"
	"testing"

	"github.com/discobox-ai/repostd/internal/canon"
	"github.com/discobox-ai/repostd/internal/repo"
)

// Every file the standard distributes on each stack, so that one silently
// dropped from the build (a .gitignore pattern matching a template, say)
// fails here.
var (
	wantManaged = map[repo.Stack][]string{
		repo.Go: {
			".agents/skills/repostd/SKILL.md",
			".github/actionlint.yaml",
			".github/actions/task/action.yml",
			".golangci.yml",
			"docs/adr/template.md",
		},
		repo.Node: {
			".agents/skills/repostd/SKILL.md",
			".github/actionlint.yaml",
			".github/actions/pnpm/action.yml",
			".github/actions/repocheck/action.yml",
			"docs/adr/template.md",
			"oxfmt.config.ts",
			"oxlint.repostd.json",
		},
	}
	wantStarter = map[repo.Stack][]string{
		repo.Go: {
			".discobox/hooks/10-gofmt.sh",
			".discobox/hooks/11-go-mod-tidy.sh",
			".discobox/hooks/80-review-test.sh",
			".discobox/hooks/90-review-check.sh",
			".envrc", ".gitattributes", ".github/workflows/ci.yml", ".gitignore",
			"AGENTS.md", "DESIGN.md", "LICENSE", "NOTICE", "README.md", "SECURITY.md",
			"Taskfile.yml", "docs/adr/README.md", "flake.nix", "renovate.json",
		},
		repo.Node: {
			".discobox/hooks/10-format.sh",
			".discobox/hooks/80-review-test.sh",
			".discobox/hooks/90-review-check.sh",
			".gitattributes", ".github/workflows/ci.yml", ".gitignore", ".oxlintrc.json",
			"AGENTS.md", "DESIGN.md", "LICENSE", "NOTICE", "README.md", "SECURITY.md",
			"docs/adr/README.md", "renovate.json",
		},
	}
)

func TestEveryDistributedFileIsInTheBuild(t *testing.T) {
	for _, stack := range []repo.Stack{repo.Go, repo.Node} {
		var managed []string
		for _, f := range canon.Managed(stack) {
			managed = append(managed, f.Path)
		}
		if !slices.Equal(managed, wantManaged[stack]) {
			t.Errorf("Managed(%s) = %v, want %v", stack, managed, wantManaged[stack])
		}
		files, err := canon.Starter(stack, canon.Vars{Module: "m", Name: "m", Year: 2026, RepocheckVersion: "main"})
		if err != nil {
			t.Fatal(err)
		}
		var starter []string
		for _, f := range files {
			starter = append(starter, f.Path)
		}
		if !slices.Equal(starter, wantStarter[stack]) {
			t.Errorf("Starter(%s) = %v, want %v", stack, starter, wantStarter[stack])
		}
	}
}
