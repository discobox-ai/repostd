// Package managed computes the content repostd owns in a repo: the managed
// files (with the repo's local blocks carried over) and the ADR index. Sync
// writes it; the managed rules compare against it.
package managed

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/discobox-ai/repostd/internal/adr"
	"github.com/discobox-ai/repostd/internal/canon"
	"github.com/discobox-ai/repostd/internal/repo"
)

// LocalBlockFiles are the managed files that carry repo-owned local blocks:
// a Go repo's golangci-lint config and a node repo's oxfmt config. A node
// repo's oxlint rules need none: its own .oxlintrc.json extends the managed
// oxlint.repostd.json.
var LocalBlockFiles = []string{".golangci.yml", "oxfmt.config.ts"}

// ADRReadme is the file holding the generated ADR index.
const ADRReadme = adr.Dir + "/README.md"

// A local block is a "repostd:local <name>" line and a "repostd:end" line,
// each a comment in its file's syntax: "#" for YAML, "//" for JavaScript.
const (
	localBegin = "repostd:local "
	localEnd   = "repostd:end"
)

var commentPrefixes = []string{"# ", "// "}

// Expected returns every file whose content repostd owns, as it should be in
// r. The ADR README is included only when it exists and has index markers.
func Expected(r *repo.Repo) ([]canon.File, error) {
	var out []canon.File
	for _, f := range canon.Managed(r.Stack) {
		if slices.Contains(LocalBlockFiles, f.Path) {
			current, _ := r.Read(f.Path)
			f.Data = MergeLocal(f.Data, current)
		}
		out = append(out, f)
	}
	if readme, ok := r.Read(ADRReadme); ok {
		data, err := adr.ReplaceIndex(readme, adr.Index(ADRs(r)))
		switch {
		case errors.Is(err, adr.ErrNoIndexMarkers):
		case err != nil:
			return nil, err
		default:
			out = append(out, canon.File{Path: ADRReadme, Data: data})
		}
	}
	return out, nil
}

// ADRs parses every ADR in r.
func ADRs(r *repo.Repo) []*adr.ADR {
	var out []*adr.ADR
	for _, p := range r.Under(adr.Dir) {
		if !adr.IsRecord(p) {
			continue
		}
		data, _ := r.Read(p)
		out = append(out, adr.Parse(p, data))
	}
	return out
}

// MergeLocal returns canonical with each "repostd:local <name>" block's body
// replaced by the body of the same-named block in current. Blocks current
// lacks stay empty; blocks canonical lacks are dropped.
func MergeLocal(canonical, current []byte) []byte {
	bodies := LocalBlocks(current)
	var b bytes.Buffer
	lines := splitLines(canonical)
	for i := 0; i < len(lines); i++ {
		b.WriteString(lines[i])
		name, ok := blockName(lines[i])
		if !ok {
			continue
		}
		// Skip the canonical body, then emit the repo's.
		for i+1 < len(lines) && !isEnd(lines[i+1]) {
			i++
		}
		b.WriteString(bodies[name])
	}
	return b.Bytes()
}

// Dropped returns the names of the non-empty local blocks in r's copy of the
// managed file p that the standard no longer has, sorted. Sync refuses to
// drop their content.
func Dropped(r *repo.Repo, p string) []string {
	current, _ := r.Read(p)
	f, ok := canon.ManagedFile(r.Stack, p)
	if !ok {
		return nil
	}
	want := LocalBlocks(f.Data)
	var out []string
	for name, body := range LocalBlocks(current) {
		if _, ok := want[name]; !ok && strings.TrimSpace(body) != "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// LocalBlocks returns the body of each named local block in data.
func LocalBlocks(data []byte) map[string]string {
	out := map[string]string{}
	lines := splitLines(data)
	for i := 0; i < len(lines); i++ {
		name, ok := blockName(lines[i])
		if !ok {
			continue
		}
		var body strings.Builder
		for i+1 < len(lines) && !isEnd(lines[i+1]) {
			i++
			body.WriteString(lines[i])
		}
		out[name] = body.String()
	}
	return out
}

func blockName(line string) (string, bool) {
	t := uncomment(line)
	if !strings.HasPrefix(t, localBegin) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(t, localBegin)), true
}

func isEnd(line string) bool {
	return uncomment(line) == localEnd
}

// uncomment returns a comment line's text, or "" for any other line.
func uncomment(line string) string {
	t := strings.TrimSpace(line)
	for _, p := range commentPrefixes {
		if rest, ok := strings.CutPrefix(t, p); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// splitLines splits data into lines that keep their newline.
func splitLines(data []byte) []string {
	s := string(data)
	var out []string
	for s != "" {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, s)
			break
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
	return out
}

// Sync writes every managed file that differs, and creates the standard's
// symlinks that are missing. It returns the paths it changed. It leaves a
// lint config alone, and fails, while a local block the standard dropped
// still has content.
func Sync(r *repo.Repo) ([]string, error) {
	want, err := Expected(r)
	if err != nil {
		return nil, err
	}
	var held []string
	var changed []string
	for _, f := range want {
		if dropped := Dropped(r, f.Path); len(dropped) > 0 {
			held = append(held, fmt.Sprintf("%s (local blocks %s)", f.Path, strings.Join(dropped, ", ")))
			continue
		}
		if got, ok := r.Read(f.Path); ok && bytes.Equal(got, f.Data) {
			continue
		}
		if err := r.Write(f.Path, f.Data); err != nil {
			return changed, err
		}
		changed = append(changed, f.Path)
	}
	for _, s := range Symlinks(r) {
		if r.Exists(s.Path) {
			continue // A wrong target or a regular file is the rules' to report.
		}
		if err := r.Symlink(s.Path, s.Target); err != nil {
			return changed, err
		}
		changed = append(changed, s.Path)
	}
	if len(held) > 0 {
		return changed, fmt.Errorf("not synced: %s are no longer in the standard; move their lines into a remaining local block, then sync", strings.Join(held, "; "))
	}
	return changed, nil
}

// Symlinks are the symlinks r should have: the standard's, plus a CLAUDE.md
// beside every nested AGENTS.md.
func Symlinks(r *repo.Repo) []canon.Symlink {
	out := append([]canon.Symlink(nil), canon.Symlinks...)
	for _, f := range r.Match("AGENTS.md") {
		if dir := path.Dir(f); dir != "." {
			out = append(out, canon.Symlink{Path: path.Join(dir, "CLAUDE.md"), Target: "AGENTS.md"})
		}
	}
	return out
}
