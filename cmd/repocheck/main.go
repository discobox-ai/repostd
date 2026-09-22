// Command repocheck checks a repository against the discobox-ai repository
// standard, and writes the files the standard distributes.
//
//	repocheck [check]   report findings; exit 1 on any unwaived error
//	repocheck sync      rewrite managed files, the ADR index, and symlinks
//	repocheck init      write missing starter files, then sync
//	repocheck rules     list every rule
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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
		fmt.Fprintln(fs.Output(), "usage: repocheck [-C dir] [check|sync|init|rules]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	cmd := "check"
	if fs.NArg() > 0 {
		cmd = fs.Arg(0)
	}
	if fs.NArg() > 1 {
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
	module := r.Name()
	if r.Mod != nil && r.Mod.Module != nil {
		module = r.Mod.Module.Mod.Path
	}
	starter, err := canon.Starter(canon.Vars{Module: module, Name: r.Name(), Year: time.Now().Year()})
	if err != nil {
		return err
	}
	for _, f := range starter {
		if r.Exists(f.Path) {
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
