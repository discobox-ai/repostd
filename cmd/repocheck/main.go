// Command repocheck checks a repository against the discobox-ai repository
// standard, and writes the files the standard distributes.
//
//	repocheck [check]   report findings; exit 1 on any unwaived error
//	repocheck sync      rewrite managed files, the ADR index, and symlinks
//	repocheck init      write missing starter files, then sync
//	repocheck rules     list every rule
//	repocheck show P    print the starter or managed file P as this repo would get it
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/discobox-ai/repostd/internal/canon"
	"github.com/discobox-ai/repostd/internal/managed"
	"github.com/discobox-ai/repostd/internal/repo"
	"github.com/discobox-ai/repostd/internal/rules"
)

// errFindings reports that check found unwaived errors; they are already
// printed.
var errFindings = errors.New("findings")

func main() {
	err := run(context.Background(), os.Args[1:], os.Stdout)
	switch {
	case errors.Is(err, errFindings):
		os.Exit(1)
	case err != nil:
		fmt.Fprintln(os.Stderr, "repocheck:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("repocheck", flag.ContinueOnError)
	root := fs.String("C", ".", "repository root")
	showWaived := fs.Bool("waived", false, "also list waived findings (check)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: repocheck [-C dir] [check|sync|init|rules|show <path>]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	cmd := "check"
	if fs.NArg() > 0 {
		cmd = fs.Arg(0)
	}
	if cmd == "show" && fs.NArg() != 2 {
		return errors.New("usage: repocheck show <path>")
	}
	if cmd != "show" && fs.NArg() > 1 {
		return fmt.Errorf("unexpected arguments after %q", cmd)
	}

	if cmd == "rules" {
		for _, r := range rules.All() {
			fmt.Fprintf(stdout, "%-28s %s\n", r.ID, r.Summary)
		}
		return nil
	}

	r, err := repo.Load(ctx, *root)
	if err != nil {
		return err
	}
	switch cmd {
	case "check":
		return check(r, stdout, *showWaived)
	case "sync":
		return sync(r, stdout)
	case "init":
		return initRepo(ctx, r, stdout)
	case "show":
		return show(r, fs.Arg(1), stdout)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func check(r *repo.Repo, stdout io.Writer, showWaived bool) error {
	var errs, warns, waived int
	for _, f := range rules.Run(r) {
		switch {
		case f.Waived():
			waived++
			if showWaived {
				fmt.Fprintf(stdout, "%s (waived: %s)\n", f, f.Waiver)
			}
			continue
		case f.Severity == rules.Warning:
			warns++
		default:
			errs++
		}
		fmt.Fprintln(stdout, f)
	}
	if errs+warns+waived > 0 {
		fmt.Fprintf(stdout, "repocheck: %d errors, %d warnings, %d waived\n", errs, warns, waived)
	}
	if errs > 0 {
		return errFindings
	}
	return nil
}

func sync(r *repo.Repo, stdout io.Writer) error {
	changed, err := managed.Sync(r)
	for _, p := range changed {
		fmt.Fprintln(stdout, "wrote", p)
	}
	return err
}

func initRepo(ctx context.Context, r *repo.Repo, stdout io.Writer) error {
	// Starter files carry the module path, so a repo without go.mod would
	// bake the directory name into the Taskfile and AGENTS.md.
	if r.Mod == nil {
		return fmt.Errorf("no go.mod in %s; run `go mod init github.com/discobox-ai/%s` first", r.Root, r.Name())
	}
	starter, err := starterFiles(r)
	if err != nil {
		return err
	}
	misnamed := rules.MisnamedDesignDocs(r)
	for _, f := range starter {
		if r.Exists(f.Path) {
			continue
		}
		// A placeholder beside the repo's real design doc would hide it;
		// docs.required-files reports the move instead.
		if f.Path == "DESIGN.md" && len(misnamed) > 0 {
			fmt.Fprintf(stdout, "skipped DESIGN.md: move %s there\n", strings.Join(misnamed, ", "))
			continue
		}
		if err := r.Write(f.Path, f.Data); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "wrote", f.Path)
	}
	// Reload so sync sees the starter files (the ADR README in particular).
	if r, err = repo.Load(ctx, r.Root); err != nil {
		return err
	}
	return sync(r, stdout)
}

// show prints what the repo would get at p: the managed content, or the
// starter template rendered for this repo, for merging by hand.
func show(r *repo.Repo, p string, stdout io.Writer) error {
	want, err := managed.Expected(r)
	if err != nil {
		return err
	}
	starter, err := starterFiles(r)
	if err != nil {
		return err
	}
	var paths []string
	for _, f := range append(want, starter...) {
		if f.Path == p {
			_, err := stdout.Write(f.Data)
			return err
		}
		paths = append(paths, f.Path)
	}
	return fmt.Errorf("no starter or managed file %q; one of: %s", p, strings.Join(paths, ", "))
}

func starterFiles(r *repo.Repo) ([]canon.File, error) {
	module := r.Name()
	if r.Mod != nil && r.Mod.Module != nil {
		module = r.Mod.Module.Mod.Path
	}
	return canon.Starter(canon.Vars{Module: module, Name: r.Name(), Year: time.Now().Year()})
}
