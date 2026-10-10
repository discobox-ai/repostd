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
	if a.ID != "0007" || a.Title != "Tests skip on an env var" || a.SupersededBy != "0009" {
		t.Errorf("parsed %+v", a)
	}
	if strings.Join(a.Supersedes, ",") != "0003,0004" {
		t.Errorf("Supersedes = %v", a.Supersedes)
	}
	if strings.Join(a.Sections, ",") != "Context,Decision,Alternatives rejected,Consequences,Deferred" {
		t.Errorf("Sections = %v; a heading inside a fence must not count", a.Sections)
	}
}

func TestParseReadsADateIDAndLinkedReferences(t *testing.T) {
	dated := "# 26-10-08-974 — IDs are dates\n\n" +
		"- **Status**: Superseded by [26-10-09-001](26-10-09-001-ids-are-something-else.md)\n" +
		"- **Date**: 2026-10-08\n" +
		"- **Supersedes**: 0003, [26-09-24-458](26-09-24-458-adr-ids.md)\n\n" +
		"## Context\n\n## Decision\n\n## Alternatives rejected\n\n## Consequences\n"
	a := adr.Parse("docs/adr/26-10-08-974-ids-are-dates.md", []byte(dated))
	if len(a.Problems) > 0 {
		t.Fatalf("problems: %v", a.Problems)
	}
	if a.ID != "26-10-08-974" || a.SupersededBy != "26-10-09-001" {
		t.Errorf("parsed %+v", a)
	}
	if strings.Join(a.Supersedes, ",") != "0003,26-09-24-458" {
		t.Errorf("Supersedes = %v", a.Supersedes)
	}
}

func TestParseJoinsAWrappedStatusAndKeepsItsNote(t *testing.T) {
	wrapped := "# 0003 — Pools are first class\n\n" +
		"- **Status**: Accepted (§4 superseded by\n" +
		"  [0029](0029-no-requests.md); the rest stands)\n" +
		"- **Date**: 2026-07-17\n\n" +
		"## Context\n\n## Decision\n\n## Alternatives rejected\n\n## Consequences\n"
	a := adr.Parse("docs/adr/0003-pools-are-first-class.md", []byte(wrapped))
	if len(a.Problems) > 0 {
		t.Fatalf("problems: %v", a.Problems)
	}
	if want := "Accepted (§4 superseded by [0029](0029-no-requests.md); the rest stands)"; a.Status != want {
		t.Errorf("Status = %q, want %q", a.Status, want)
	}
	if a.Date != "2026-07-17" || a.SupersededBy != "" {
		t.Errorf("parsed %+v; a note about a part does not supersede the ADR", a)
	}
}

func TestParseReadsSupersededByWithANote(t *testing.T) {
	a := adr.Parse("docs/adr/0015-nri.md", []byte("# 0015 — NRI\n\n- **Status**: Superseded by 0020 (the plugin was never shipped)\n- **Date**: 2026-07-20\n"))
	if a.SupersededBy != "0020" {
		t.Errorf("SupersededBy = %q, want 0020", a.SupersededBy)
	}
}

func TestParseRejectsAStatusThatIsNotAState(t *testing.T) {
	for _, status := range []string{"Done", "Accepted, mostly", "Accepted (unclosed"} {
		a := adr.Parse("docs/adr/0001-a.md", []byte("# 0001 — A\n\n- **Status**: "+status+"\n- **Date**: 2026-07-20\n"))
		if !strings.Contains(strings.Join(a.Problems, "\n"), "status ") {
			t.Errorf("status %q: no status problem in %v", status, a.Problems)
		}
	}
}

func TestParseRejectsAnIDOfNeitherForm(t *testing.T) {
	for _, name := range []string{"007-short.md", "26-10-8-974-one-digit-day.md", "26-10-08-97-short-suffix.md"} {
		a := adr.Parse("docs/adr/"+name, []byte("# 007 — Short\n"))
		if a.ID != "" || len(a.Problems) == 0 {
			t.Errorf("%s: ID %q, problems %v; want no ID and a problem", name, a.ID, a.Problems)
		}
	}
}

func TestIndexSortsSequentialIDsBeforeDateIDs(t *testing.T) {
	var adrs []*adr.ADR
	for _, f := range []string{"26-10-08-974-b.md", "0002-a.md", "26-09-24-458-c.md"} {
		adrs = append(adrs, adr.Parse("docs/adr/"+f, nil))
	}
	got := adr.Index(adrs)
	want := []string{"[0002]", "[26-09-24-458]", "[26-10-08-974]"}
	last := -1
	for _, w := range want {
		i := strings.Index(got, w)
		if i <= last {
			t.Fatalf("index out of order, want %v:\n%s", want, got)
		}
		last = i
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
