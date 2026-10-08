// Package repo is the view of a repository that rules check: its files, as
// git sees them, and its waivers.
package repo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/modfile"
)

// WaiverFile is the repo-relative path of the waiver file.
const WaiverFile = ".repocheck.yaml"

// Stack is the toolchain a repo is built with, which decides the toolchain
// rules it follows (ADR 0002).
type Stack string

const (
	// Go is a repo with a go.mod, and a repo with neither manifest yet.
	Go Stack = "go"
	// Node is a repo with a package.json and no go.mod.
	Node Stack = "node"
)

// Package is the part of a package.json the node rules read.
type Package struct {
	Name           string            `json:"name"`
	PackageManager string            `json:"packageManager"`
	Scripts        map[string]string `json:"scripts"`
	DevEngines     struct {
		Runtime json.RawMessage `json:"runtime"`
	} `json:"devEngines"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// Depends reports whether the package depends on name, at runtime or for
// development.
func (p *Package) Depends(name string) bool {
	_, dep := p.Dependencies[name]
	_, dev := p.DevDependencies[name]
	return dep || dev
}

// Repo is a repository rooted at Root.
type Repo struct {
	Root string
	// Files are the tracked and untracked-but-not-ignored files that exist on
	// disk, as sorted slash-separated paths relative to Root. Symlinks are
	// listed as files.
	Files []string
	// Mod is the parsed root go.mod, or nil when there is none.
	Mod *modfile.File
	// Package is the parsed root package.json, or nil when there is none.
	Package *Package
	// Stack is detected from the manifests: Node for a package.json without
	// a go.mod, Go otherwise.
	Stack   Stack
	Waivers []Waiver
}

// Waiver exempts findings of Rule, optionally only under the Path glob.
type Waiver struct {
	Rule   string `yaml:"rule"`
	Path   string `yaml:"path,omitempty"`
	Reason string `yaml:"reason"`
}

// Load reads the git repository containing dir. dir may be any directory
// inside it.
func Load(ctx context.Context, dir string) (*Repo, error) {
	abs, err := toplevel(ctx, dir)
	if err != nil {
		return nil, err
	}
	r := &Repo{Root: abs}
	if r.Files, err = listFiles(ctx, abs); err != nil {
		return nil, err
	}
	if data, ok := r.Read("go.mod"); ok {
		if r.Mod, err = modfile.Parse("go.mod", data, nil); err != nil {
			return nil, err
		}
	}
	if data, ok := r.Read("package.json"); ok {
		r.Package = &Package{}
		if err := json.Unmarshal(data, r.Package); err != nil {
			return nil, fmt.Errorf("package.json: %w", err)
		}
	}
	r.Stack = Go
	if r.Mod == nil && r.Package != nil {
		r.Stack = Node
	}
	if data, ok := r.Read(WaiverFile); ok {
		var cfg struct {
			Waivers []Waiver `yaml:"waivers"`
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", WaiverFile, err)
		}
		for _, w := range cfg.Waivers {
			if w.Rule == "" || strings.TrimSpace(w.Reason) == "" {
				return nil, fmt.Errorf("%s: every waiver needs a rule and a reason", WaiverFile)
			}
		}
		r.Waivers = cfg.Waivers
	}
	return r, nil
}

// toplevel returns the root of the git work tree containing dir.
func toplevel(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		abs, _ := filepath.Abs(dir)
		return "", fmt.Errorf("%s is not inside a git repository", abs)
	}
	return filepath.FromSlash(strings.TrimSpace(string(out))), nil
}

func listFiles(ctx context.Context, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w: %s", root, err, strings.TrimSpace(stderr.String()))
	}
	seen := map[string]bool{}
	var files []string
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		// Deleted but still in the index.
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			continue
		}
		files = append(files, p)
	}
	sort.Strings(files)
	return files, nil
}

// Name is the repo name: the module path's last element, a node repo's
// package name without its scope, or the directory name.
func (r *Repo) Name() string {
	if r.Mod != nil && r.Mod.Module != nil {
		return path.Base(r.Mod.Module.Mod.Path)
	}
	if r.Stack == Node && r.Package.Name != "" {
		return path.Base(r.Package.Name)
	}
	return filepath.Base(r.Root)
}

// Abs returns the absolute path of the repo-relative path p.
func (r *Repo) Abs(p string) string {
	return filepath.Join(r.Root, filepath.FromSlash(p))
}

// Has reports whether p is a listed file.
func (r *Repo) Has(p string) bool {
	i := sort.SearchStrings(r.Files, p)
	return i < len(r.Files) && r.Files[i] == p
}

// Read returns the contents of p, following symlinks, if p exists on disk.
func (r *Repo) Read(p string) ([]byte, bool) {
	data, err := os.ReadFile(r.Abs(p))
	if err != nil {
		return nil, false
	}
	return data, true
}

// Readlink returns the target of p when p is a symlink.
func (r *Repo) Readlink(p string) (string, bool) {
	fi, err := os.Lstat(r.Abs(p))
	if err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		return "", false
	}
	target, err := os.Readlink(r.Abs(p))
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(target), true
}

// Exists reports whether p exists on disk as anything, including a broken
// symlink.
func (r *Repo) Exists(p string) bool {
	_, err := os.Lstat(r.Abs(p))
	return err == nil
}

// Match returns the listed files whose base name matches the glob pattern.
func (r *Repo) Match(pattern string) []string {
	var out []string
	for _, f := range r.Files {
		if ok, _ := path.Match(pattern, path.Base(f)); ok {
			out = append(out, f)
		}
	}
	return out
}

// Under returns the listed files under dir.
func (r *Repo) Under(dir string) []string {
	prefix := strings.TrimSuffix(dir, "/") + "/"
	var out []string
	for _, f := range r.Files {
		if strings.HasPrefix(f, prefix) {
			out = append(out, f)
		}
	}
	return out
}

// Write writes data to p, creating parent directories. A shell script is
// written executable, because a hook or service is run, not sourced.
func (r *Repo) Write(p string, data []byte) error {
	abs := r.Abs(p)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if path.Ext(p) == ".sh" {
		mode = 0o755
	}
	return os.WriteFile(abs, data, mode) //nolint:gosec // G306: repo files are world-readable by design.
}

// Symlink creates p pointing at target. It fails if p exists.
func (r *Repo) Symlink(p, target string) error {
	abs := r.Abs(p)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if err := os.Symlink(filepath.FromSlash(target), abs); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%s already exists", p)
		}
		return err
	}
	return nil
}

// Waived reports whether a finding of rule at p is waived, and why.
func (r *Repo) Waived(rule, p string) (string, bool) {
	for _, w := range r.Waivers {
		if w.Rule != rule {
			continue
		}
		if w.Path == "" {
			return w.Reason, true
		}
		if ok, _ := path.Match(w.Path, p); ok || strings.HasPrefix(p, strings.TrimSuffix(w.Path, "/")+"/") {
			return w.Reason, true
		}
	}
	return "", false
}
