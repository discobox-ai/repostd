// Package canon holds the files the standard distributes.
//
// Managed files are owned by repostd: repocheck sync rewrites them and
// repocheck reports any drift. Starter files are written once by repocheck
// init, rendered with the repo's name, and owned by the repo afterwards.
//
// Both are split by stack (ADR 0002): files/<kind>/common is every repo's,
// and files/<kind>/go and files/<kind>/node are added for that stack.
package canon

import (
	"bytes"
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"
	"text/template"

	"github.com/discobox-ai/repostd/internal/repo"
)

//go:embed all:files
var files embed.FS

// File is one distributed file, by its slash-separated path in a repo.
type File struct {
	Path string
	Data []byte
}

// Symlink is a symlink the standard requires, by its path in a repo.
type Symlink struct {
	Path   string
	Target string
}

// Vars are the values starter templates are rendered with.
type Vars struct {
	// Module is the Go module path; a node repo's is its name.
	Module string
	// Name is the repo and binary name: the module path's last element, or
	// a node repo's package name.
	Name string
	// Year is the copyright year.
	Year int
	// RepocheckVersion is the repocheck a node repo's CI runs: the version
	// of the repocheck writing the file, or main when it has none.
	RepocheckVersion string
}

// Symlinks are the symlinks every repo has.
var Symlinks = []Symlink{
	{Path: "CLAUDE.md", Target: "AGENTS.md"},
	{Path: ".claude/skills", Target: "../.agents/skills"},
}

// Managed returns the managed files a repo on stack has, sorted by path.
func Managed(stack repo.Stack) []File {
	return forStack("files/managed", stack)
}

// ManagedFile returns the managed file at p for stack.
func ManagedFile(stack repo.Stack, p string) (File, bool) {
	for _, f := range Managed(stack) {
		if f.Path == p {
			return f, true
		}
	}
	return File{}, false
}

// Starter returns the starter files for stack rendered with v, sorted by
// path.
func Starter(stack repo.Stack, v Vars) ([]File, error) {
	raw := forStack("files/starter", stack)
	out := make([]File, 0, len(raw))
	for _, f := range raw {
		// "[[ ]]" delimiters, because Taskfiles and workflows are full of
		// "{{ }}" that belong to them.
		t, err := template.New(f.Path).Delims("[[", "]]").Option("missingkey=error").Parse(string(f.Data))
		if err != nil {
			return nil, err
		}
		var b bytes.Buffer
		if err := t.Execute(&b, v); err != nil {
			return nil, err
		}
		out = append(out, File{Path: f.Path, Data: b.Bytes()})
	}
	return out, nil
}

// forStack returns kind's common files and stack's, sorted by path.
func forStack(kind string, stack repo.Stack) []File {
	out := append(load(kind+"/common"), load(kind+"/"+string(stack))...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func load(root string) []File {
	var out []File
	err := fs.WalkDir(files, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := files.ReadFile(p)
		if err != nil {
			return err
		}
		// Every embedded file carries a .tmpl suffix so that repostd's own
		// tooling (git, direnv, repocheck) does not mistake the templates for
		// this repo's AGENTS.md, .gitignore, .envrc, and so on.
		rel, _ := relPath(root, p)
		out = append(out, File{Path: strings.TrimSuffix(rel, ".tmpl"), Data: data})
		return nil
	})
	if err != nil {
		// The tree is embedded at build time; failing to walk it is a bug.
		panic(err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func relPath(root, p string) (string, bool) {
	prefix := path.Clean(root) + "/"
	if len(p) < len(prefix) || p[:len(prefix)] != prefix {
		return p, false
	}
	return p[len(prefix):], true
}
