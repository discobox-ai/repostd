// Package canon holds the files the standard distributes.
//
// Managed files are owned by repostd: repocheck sync rewrites them and
// repocheck reports any drift. Starter files are written once by repocheck
// init, rendered with the repo's name, and owned by the repo afterwards.
package canon

import (
	"bytes"
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"
	"text/template"
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
	// Module is the Go module path.
	Module string
	// Name is the repo and binary name: the module path's last element.
	Name string
	// Year is the copyright year.
	Year int
}

// Symlinks are the symlinks every repo has.
var Symlinks = []Symlink{
	{Path: "CLAUDE.md", Target: "AGENTS.md"},
	{Path: ".claude/skills", Target: "../.agents/skills"},
}

// Managed returns the managed files, sorted by path.
func Managed() []File {
	return load("files/managed")
}

// ManagedFile returns the managed file at p.
func ManagedFile(p string) (File, bool) {
	for _, f := range Managed() {
		if f.Path == p {
			return f, true
		}
	}
	return File{}, false
}

// Starter returns the starter files rendered with v, sorted by path.
func Starter(v Vars) ([]File, error) {
	raw := load("files/starter")
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
