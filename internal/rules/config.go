package rules

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/discobox-ai/repostd/internal/repo"
)

// ServerFlags are the only flags a server binary may take. None of them is a
// setting: two ask the binary about itself, and `config` says where the
// settings are.
var ServerFlags = []string{"config", "c", "check-config", "version", "v"}

var (
	// serverDirRE matches a server binary's directory, cmd/<name>-server,
	// at the repo root or in a module subdirectory.
	serverDirRE = regexp.MustCompile(`(^|/)cmd/[a-z0-9][a-z0-9-]*-server$`)
	// flagRE matches a flag registration and captures the flag's name: the
	// standard library's flag.String / flags.StringVar / flag.Var, and
	// pflag's Flags().DurationVarP, as cobra uses it. The type names are
	// listed so that flag.NewFlagSet("name") is not mistaken for one.
	flagRE = regexp.MustCompile(`\.(?:String|Bool|Int|Int8|Int16|Int32|Int64|Uint|Uint8|Uint16|Uint32|Uint64|Float32|Float64|Duration|Func|BoolFunc|Text|Var|StringSlice|StringArray|IntSlice|BoolSlice|DurationSlice|Count|IP|IPNet|IPMask|BytesHex|BytesBase64)(?:Var)?P?\(\s*(?:&?[A-Za-z0-9_.\[\]]+\s*,\s*)?"([a-z0-9][a-z0-9-]*)"`)
)

var configRules = []Rule{
	{
		ID:      "config.no-flags",
		Summary: "A server binary takes no configuration flags, only --config, --check-config and --version.",
		Check: func(r *repo.Repo) []Issue {
			var out []Issue
			for _, dir := range serverDirs(r) {
				for _, f := range r.Under(dir) {
					if path.Ext(f) != ".go" || strings.HasSuffix(f, "_test.go") {
						continue
					}
					data, _ := r.Read(f)
					for _, m := range flagRE.FindAllStringSubmatch(string(data), -1) {
						if !slices.Contains(ServerFlags, m[1]) {
							out = append(out, issue(f, "flag %q: a server is configured by its file and the environment, not by flags", m[1]))
						}
					}
				}
			}
			return out
		},
	},
	{
		ID:      "config.schema",
		Summary: "A server commits a generated config schema and example file.",
		Check: func(r *repo.Repo) []Issue {
			dirs := serverDirs(r)
			if len(dirs) == 0 {
				return nil
			}
			var out []Issue
			if len(r.Match("*.schema.json")) == 0 {
				out = append(out, issue(dirs[0], "no *.schema.json: generate the config schema from the Config struct and commit it"))
			}
			if len(r.Match("*.example.yaml")) == 0 && len(r.Match("*.example.yml")) == 0 {
				out = append(out, issue(dirs[0], "no *.example.yaml: generate the commented example config and commit it"))
			}
			return out
		},
	},
}

// serverDirs returns the cmd/<name>-server directories, sorted.
func serverDirs(r *repo.Repo) []string {
	var out []string
	for _, f := range r.Files {
		dir := path.Dir(f)
		if serverDirRE.MatchString(dir) && !slices.Contains(out, dir) {
			out = append(out, dir)
		}
	}
	slices.Sort(out)
	return out
}
