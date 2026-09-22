package adr_test

import (
	"strings"
	"testing"

	"github.com/discobox-ai/repostd/internal/adr"
)

const record = `# 0007 — Tests skip on an env var

- **Status**: Superseded by 0009
- **Date**: 2026-09-22
- **Supersedes**: 0003, 0004

## Context

` + "```md\n## Not a section\n```" + `

## Decision

## Alternatives rejected

## Consequences

## Deferred
`

func TestParseReadsTheStandardHeaderAndSections(t *testing.T) {
	a := adr.Parse("docs/adr/0007-tests-skip-on-an-env-var.md", []byte(record))
	if len(a.Problems) > 0 {
		t.Fatalf("problems: %v", a.Problems)
	}
	if a.Number != "0007" || a.Title != "Tests skip on an env var" || a.SupersededBy != "0009" {
		t.Errorf("parsed %+v", a)
	}
	if strings.Join(a.Supersedes, ",") != "0003,0004" {
		t.Errorf("Supersedes = %v", a.Supersedes)
	}
	if strings.Join(a.Sections, ",") != "Context,Decision,Alternatives rejected,Consequences,Deferred" {
		t.Errorf("Sections = %v; a heading inside a fence must not count", a.Sections)
	}
}

func TestParseReportsTheOldDiscoboxHeaderStyle(t *testing.T) {
	old := "# 0066. Thin triggers\n\nStatus: Accepted\nDate: 2026-01-01\n"
	a := adr.Parse("docs/adr/0066-thin-triggers.md", []byte(old))
	if len(a.Problems) == 0 {
		t.Error("no problems for a non-standard header")
	}
}

func TestReplaceIndexRewritesOnlyBetweenTheMarkers(t *testing.T) {
	readme := "# ADRs\n\n" + adr.IndexBegin + "\nstale\n" + adr.IndexEnd + "\ntrailer\n"
	a := adr.Parse("docs/adr/0001-a.md", []byte("# 0001 — A | B\n\n- **Status**: Accepted\n- **Date**: 2026-09-22\n"))
	got, err := adr.ReplaceIndex([]byte(readme), adr.Index([]*adr.ADR{a}))
	if err != nil {
		t.Fatal(err)
	}
	want := "# ADRs\n\n" + adr.IndexBegin + "\n| ADR | Title | Status |\n|---|---|---|\n| [0001](0001-a.md) | A \\| B | Accepted |\n" + adr.IndexEnd + "\ntrailer\n"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
