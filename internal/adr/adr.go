// Package adr parses ADRs in the standard's format and renders their index.
package adr

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Dir is the repo-relative ADR directory.
const Dir = "docs/adr"

// Index markers in docs/adr/README.md. The lines between them are generated.
const (
	IndexBegin = "<!-- repostd:adr-index -->"
	IndexEnd   = "<!-- /repostd:adr-index -->"
)

// RequiredSections are the "##" sections every ADR has, in order.
var RequiredSections = []string{"Context", "Decision", "Alternatives rejected", "Consequences"}

// OptionalSections may appear in addition to RequiredSections.
var OptionalSections = []string{"Deferred"}

// id matches an ADR ID: YY-MM-DD-RRR, the date the ADR was written plus three
// random digits, or NNNN, the sequential form ADRs written before it keep.
const id = `\d{2}-\d{2}-\d{2}-\d{3}|\d{4}`

// ref matches a reference to an ADR: its bare ID, or a Markdown link whose
// text is the ID.
const ref = `(?:(` + id + `)|\[(` + id + `)\]\([^)\s]+\))`

var (
	fileRE  = regexp.MustCompile(`^(` + id + `)-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$`)
	titleRE = regexp.MustCompile(`^# (` + id + `) — (\S.*)$`)
	fieldRE = regexp.MustCompile(`^- \*\*([A-Za-z ]+)\*\*: (.*)$`)
	// A status is one of the four states, then an optional note in parentheses
	// saying which part of the ADR another one superseded, refined or narrowed.
	statusRE = regexp.MustCompile(`^(Proposed|Accepted|Rejected|Superseded by ` + ref + `)(?: \((.+)\))?$`)
	refRE    = regexp.MustCompile(`^` + ref + `$`)
	dateRE   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// refID returns the ADR ID in the submatches of ref, which are bare or linked.
func refID(m []string) string {
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// ADR is one parsed record.
type ADR struct {
	Path string
	// ID is YY-MM-DD-RRR, or NNNN for an ADR written before that form.
	ID     string
	Title  string
	Status string
	Date   string
	// SupersededBy is set when Status is "Superseded by <ID>".
	SupersededBy string
	Supersedes   []string
	Sections     []string
	// Problems are format violations found while parsing.
	Problems []string
}

// IsRecord reports whether the repo-relative path p is an ADR file (as
// opposed to the README or template).
func IsRecord(p string) bool {
	if path.Dir(p) != Dir || path.Ext(p) != ".md" {
		return false
	}
	base := path.Base(p)
	return base != "README.md" && base != "template.md"
}

// Parse parses the ADR at the repo-relative path p.
func Parse(p string, data []byte) *ADR {
	a := &ADR{Path: p}
	m := fileRE.FindStringSubmatch(path.Base(p))
	if m == nil {
		a.Problems = append(a.Problems, "filename is not <ID>-lowercase-hyphenated-slug.md, where <ID> is YY-MM-DD-RRR or NNNN")
	} else {
		a.ID = m[1]
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i == len(lines) {
		a.Problems = append(a.Problems, "file is empty")
		return a
	}
	if t := titleRE.FindStringSubmatch(lines[i]); t == nil {
		a.Problems = append(a.Problems, fmt.Sprintf("first line is not \"# <ID> — Title\": %q", lines[i]))
	} else {
		if a.ID != "" && t[1] != a.ID {
			a.Problems = append(a.Problems, fmt.Sprintf("title ID %s does not match filename ID %s", t[1], a.ID))
		}
		if a.ID == "" {
			a.ID = t[1]
		}
		a.Title = t[2]
	}
	i++

	// Header fields: consecutive "- **Field**: value" lines after the title.
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	fields := map[string]string{}
	for i < len(lines) {
		f := fieldRE.FindStringSubmatch(lines[i])
		if f == nil {
			break
		}
		value := []string{strings.TrimSpace(f[2])}
		// A value wraps onto the indented lines that follow it.
		for i++; i < len(lines) && isContinuation(lines[i]); i++ {
			value = append(value, strings.TrimSpace(lines[i]))
		}
		fields[f[1]] = strings.TrimSpace(strings.Join(value, " "))
	}
	a.Status = fields["Status"]
	a.Date = fields["Date"]
	switch s := statusRE.FindStringSubmatch(a.Status); {
	case a.Status == "":
		a.Problems = append(a.Problems, "missing \"- **Status**:\" header line")
	case s == nil:
		a.Problems = append(a.Problems, fmt.Sprintf("status %q is not Proposed, Accepted, Rejected, or \"Superseded by <ID>\", optionally followed by a note in parentheses", a.Status))
	case strings.HasPrefix(s[1], "Superseded by "):
		a.SupersededBy = refID(s[1:])
	}
	if !dateRE.MatchString(a.Date) {
		a.Problems = append(a.Problems, "missing or malformed \"- **Date**: YYYY-MM-DD\" header line")
	}
	if s, ok := fields["Supersedes"]; ok {
		for n := range strings.SplitSeq(s, ",") {
			m := refRE.FindStringSubmatch(strings.TrimSpace(n))
			if m == nil {
				a.Problems = append(a.Problems, fmt.Sprintf("Supersedes entry %q is not an ADR ID or a link to one", strings.TrimSpace(n)))
				continue
			}
			a.Supersedes = append(a.Supersedes, refID(m))
		}
	}

	inFence := false
	for _, l := range lines[i:] {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inFence = !inFence
			continue
		}
		if !inFence && strings.HasPrefix(l, "## ") {
			a.Sections = append(a.Sections, strings.TrimSpace(strings.TrimPrefix(l, "## ")))
		}
	}
	a.Problems = append(a.Problems, sectionProblems(a.Sections)...)
	return a
}

// isContinuation reports whether l continues the header field above it: it is
// indented and not blank.
func isContinuation(l string) bool {
	return strings.TrimSpace(l) != "" && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t"))
}

func sectionProblems(got []string) []string {
	var problems []string
	allowed := map[string]bool{}
	for _, s := range append(append([]string{}, RequiredSections...), OptionalSections...) {
		allowed[s] = true
	}
	have := map[string]bool{}
	for _, s := range got {
		have[s] = true
		if !allowed[s] {
			problems = append(problems, fmt.Sprintf("section %q is not one of the standard sections", s))
		}
	}
	for _, s := range RequiredSections {
		if !have[s] {
			problems = append(problems, fmt.Sprintf("missing section \"## %s\"", s))
		}
	}
	return problems
}

// Index renders the index table for adrs, ordered by ID: the NNNN ADRs sort
// first, then the YY-MM-DD-RRR ones by date.
func Index(adrs []*ADR) string {
	sorted := append([]*ADR(nil), adrs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ID != sorted[j].ID {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Path < sorted[j].Path
	})
	var b strings.Builder
	b.WriteString("| ADR | Title | Status |\n|---|---|---|\n")
	for _, a := range sorted {
		cell := strings.NewReplacer("|", `\|`)
		fmt.Fprintf(&b, "| [%s](%s) | %s | %s |\n", a.ID, path.Base(a.Path), cell.Replace(a.Title), cell.Replace(a.Status))
	}
	return b.String()
}

// ErrNoIndexMarkers is returned when a README lacks the index markers.
var ErrNoIndexMarkers = errors.New("no " + IndexBegin + " / " + IndexEnd + " markers")

// ReplaceIndex returns readme with the lines between the index markers
// replaced by index.
func ReplaceIndex(readme []byte, index string) ([]byte, error) {
	begin := bytes.Index(readme, []byte(IndexBegin))
	end := bytes.Index(readme, []byte(IndexEnd))
	if begin < 0 || end < begin {
		return nil, ErrNoIndexMarkers
	}
	var b bytes.Buffer
	b.Write(readme[:begin+len(IndexBegin)])
	b.WriteString("\n")
	b.WriteString(index)
	b.Write(readme[end:])
	return b.Bytes(), nil
}
