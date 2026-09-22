// Package rules holds the checks of the discobox-ai repository standard. Each
// rule ID is cited next to its rule text in the repostd skill.
package rules

import (
	"fmt"
	"sort"

	"github.com/discobox-ai/repostd/internal/repo"
)

// Severity is how bad a finding is. Warnings never fail a check.
type Severity int

const (
	// Error findings fail repocheck.
	Error Severity = iota
	// Warning findings are reported but do not fail repocheck.
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Rule is one check.
type Rule struct {
	ID      string
	Summary string
	Check   func(*repo.Repo) []Issue
}

// Issue is what a rule reports; Run turns it into a Finding.
type Issue struct {
	Path     string
	Message  string
	Severity Severity
}

// Finding is an issue attributed to its rule, after waivers.
type Finding struct {
	Rule string
	Issue
	// Waiver is the reason from .repocheck.yaml when the finding is waived.
	Waiver string
}

// Waived reports whether the finding is covered by a waiver.
func (f Finding) Waived() bool { return f.Waiver != "" }

func (f Finding) String() string {
	loc := f.Path
	if loc == "" {
		loc = "."
	}
	return fmt.Sprintf("%s: %s [%s] %s", loc, f.Severity, f.Rule, f.Message)
}

func issue(p, format string, args ...any) Issue {
	return Issue{Path: p, Message: fmt.Sprintf(format, args...)}
}

func warning(p, format string, args ...any) Issue {
	return Issue{Path: p, Message: fmt.Sprintf(format, args...), Severity: Warning}
}

// All returns every rule, sorted by ID.
func All() []Rule {
	var all []Rule
	for _, group := range [][]Rule{layoutRules, docsRules, adrRules, agentsRules, envRules, lintRules, testsRules, ciRules, gitRules} {
		all = append(all, group...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return all
}

// Run checks r against every rule and applies its waivers.
func Run(r *repo.Repo) []Finding {
	var out []Finding
	for _, rule := range All() {
		for _, is := range rule.Check(r) {
			f := Finding{Rule: rule.ID, Issue: is}
			f.Waiver, _ = r.Waived(rule.ID, is.Path)
			out = append(out, f)
		}
	}
	return out
}
