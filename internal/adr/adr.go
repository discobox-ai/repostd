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

var (
	fileRE       = regexp.MustCompile(`^(\d{4})-([a-z0-9]+(?:-[a-z0-9]+)*)\.md$`)
	titleRE      = regexp.MustCompile(`^# (\d{4}) — (\S.*)$`)
	fieldRE      = regexp.MustCompile(`^- \*\*([A-Za-z ]+)\*\*: (.*)$`)
	supersededRE = regexp.MustCompile(`^Superseded by (\d{4})$`)
	numberRE     = regexp.MustCompile(`^\d{4}$`)
	dateRE       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// ADR is one parsed record.
type ADR struct {
	Path   string
	Number string
	Title  string
	Status string
	Date   string
	// SupersededBy is set when Status is "Superseded by NNNN".
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
		a.Problems = append(a.Problems, "filename is not NNNN-lowercase-hyphenated-slug.md")
	} else {
		a.Number = m[1]
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
		a.Problems = append(a.Problems, fmt.Sprintf("first line is not \"# NNNN — Title\": %q", lines[i]))
	} else {
		if a.Number != "" && t[1] != a.Number {
			a.Problems = append(a.Problems, fmt.Sprintf("title number %s does not match filename number %s", t[1], a.Number))
		}
		if a.Number == "" {
			a.Number = t[1]
		}
		a.Title = t[2]
	}
	i++

	// Header fields: consecutive "- **Field**: value" lines after the title.
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	fields := map[string]string{}
	for ; i < len(lines); i++ {
		f := fieldRE.FindStringSubmatch(lines[i])
		if f == nil {
			break
		}
		fields[f[1]] = strings.TrimSpace(f[2])
	}
	a.Status = fields["Status"]
	a.Date = fields["Date"]
	switch a.Status {
	case "":
		a.Problems = append(a.Problems, "missing \"- **Status**:\" header line")
	case "Proposed", "Accepted", "Rejected":
	default:
		if s := supersededRE.FindStringSubmatch(a.Status); s != nil {
			a.SupersededBy = s[1]
		} else {
			a.Problems = append(a.Problems, fmt.Sprintf("status %q is not Proposed, Accepted, Rejected, or \"Superseded by NNNN\"", a.Status))
		}
	}
	if !dateRE.MatchString(a.Date) {
		a.Problems = append(a.Problems, "missing or malformed \"- **Date**: YYYY-MM-DD\" header line")
	}
	if s, ok := fields["Supersedes"]; ok {
		for n := range strings.SplitSeq(s, ",") {
			n = strings.TrimSpace(n)
			if !numberRE.MatchString(n) {
				a.Problems = append(a.Problems, fmt.Sprintf("Supersedes entry %q is not an ADR number", n))
				continue
			}
			a.Supersedes = append(a.Supersedes, n)
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

// Index renders the index table for adrs, ordered by number.
func Index(adrs []*ADR) string {
	sorted := append([]*ADR(nil), adrs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Number != sorted[j].Number {
			return sorted[i].Number < sorted[j].Number
		}
		return sorted[i].Path < sorted[j].Path
	})
	var b strings.Builder
	b.WriteString("| ADR | Title | Status |\n|---|---|---|\n")
	for _, a := range sorted {
		title := strings.ReplaceAll(a.Title, "|", `\|`)
		fmt.Fprintf(&b, "| [%s](%s) | %s | %s |\n", a.Number, path.Base(a.Path), title, a.Status)
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
