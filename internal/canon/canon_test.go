package canon_test

import (
	"slices"
	"testing"

	"github.com/discobox-ai/repostd/internal/canon"
)

// Every file the standard distributes, so that one silently dropped from the
// build (a .gitignore pattern matching a template, say) fails here.
var (
	wantManaged = []string{
		".agents/skills/repostd/SKILL.md",
		".github/actionlint.yaml",
		".github/actions/task/action.yml",
		".golangci.yml",
		"docs/adr/template.md",
	}
	wantStarter = []string{
		".envrc", ".gitattributes", ".github/workflows/ci.yml", ".gitignore",
		"AGENTS.md", "DESIGN.md", "LICENSE", "NOTICE", "README.md", "SECURITY.md",
		"Taskfile.yml", "docs/adr/README.md", "flake.nix", "renovate.json",
	}
)

func TestEveryDistributedFileIsInTheBuild(t *testing.T) {
	var managed []string
	for _, f := range canon.Managed() {
		managed = append(managed, f.Path)
	}
	if !slices.Equal(managed, wantManaged) {
		t.Errorf("Managed() = %v, want %v", managed, wantManaged)
	}
	files, err := canon.Starter(canon.Vars{Module: "m", Name: "m", Year: 2026})
	if err != nil {
		t.Fatal(err)
	}
	var starter []string
	for _, f := range files {
		starter = append(starter, f.Path)
	}
	if !slices.Equal(starter, wantStarter) {
		t.Errorf("Starter() = %v, want %v", starter, wantStarter)
	}
}
