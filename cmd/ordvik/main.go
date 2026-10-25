// Command ordvik reads and checks planning data: dispatch docs, plans and
// operator asks, in a code repository's docs/planning/ or in the planning
// repository.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/xythian/ordvik/planning"
)

const usage = `ordvik planning: read and check dispatch docs, plans and asks.

Usage:
  ordvik planning status <plan> [--json]   The plan's threads by stage, and what to do next
  ordvik planning ready <plan>             Paths of launchable dispatch docs, one per line
  ordvik planning check [<plan>]           Validate the plan (or only dispatch ids) and ask files
  ordvik planning asks [--json] [--all] [--today YYYY-MM-DD] [--project <p>]
                                           What is waiting on the operator
  ordvik planning asks check               Validate ask files and gate entries
  ordvik planning projects [--json]        The projects and their repositories

Every command takes:
  --ref <ref>   Read dispatch docs from <ref> (default origin/HEAD, else origin/main or master)
  --fetch       git fetch origin first
  --offline     In the planning repository, do not fetch (it fetches by default)
  --worktree    Read everything from the working tree (the default for check
                in the planning repository, so an edit is checked before it is committed)

<plan> is a path, <project>/<slug>, or a slug under a project's plans/.

In a code repository, docs/planning/ is read as one project, the way
scripts/plan and scripts/asks read it: dispatch docs from the ref, plans
and ask files from the working tree.

ordvik plan ... and ordvik asks ... are scripts/plan and scripts/asks.

Exit status: 0, 1 on errors (check finds any, or the store cannot be read), 2 on bad usage.
`

type opts struct {
	ref      string
	fetch    bool
	offline  bool
	worktree bool
	json     bool
	all      bool
	today    string
	project  string
	args     []string
}

func parse(argv []string) (opts, error) {
	var o opts
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		takes := func() (string, error) {
			if i+1 >= len(argv) {
				return "", fmt.Errorf("%s takes a value", a)
			}
			i++
			return argv[i], nil
		}
		var err error
		switch a {
		case "--ref":
			o.ref, err = takes()
		case "--today":
			o.today, err = takes()
		case "--project":
			o.project, err = takes()
		case "--fetch":
			o.fetch = true
		case "--offline":
			o.offline = true
		case "--worktree":
			o.worktree = true
		case "--json":
			o.json = true
		case "--all":
			o.all = true
		default:
			if strings.HasPrefix(a, "--") {
				return o, fmt.Errorf("unexpected argument %q", a)
			}
			o.args = append(o.args, a)
		}
		if err != nil {
			return o, err
		}
	}
	return o, nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// dir is where the store is looked for; tests set it.
var dir = "."

func run(argv []string, stdout, stderr io.Writer) int {
	if len(argv) == 0 || argv[0] == "-h" || argv[0] == "--help" || argv[0] == "help" {
		fmt.Fprint(stdout, usage)
		if len(argv) == 0 {
			return 2
		}
		return 0
	}
	switch argv[0] {
	case "planning":
		return planningCmd(argv[1:], stdout, stderr, false)
	case "plan":
		return planningCmd(argv[1:], stdout, stderr, false)
	case "asks":
		return planningCmd(append([]string{"asks"}, argv[1:]...), stdout, stderr, false)
	}
	fmt.Fprintf(stderr, "ordvik: unknown command %q\n\n%s", argv[0], usage)
	return 2
}

func planningCmd(argv []string, stdout, stderr io.Writer, _ bool) int {
	if len(argv) == 0 || argv[0] == "-h" || argv[0] == "--help" || argv[0] == "help" {
		fmt.Fprint(stdout, usage)
		if len(argv) == 0 {
			return 2
		}
		return 0
	}
	cmd := argv[0]
	o, err := parse(argv[1:])
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	if cmd == "asks" && len(o.args) > 0 && (o.args[0] == "status" || o.args[0] == "check") {
		if o.args[0] == "check" {
			cmd = "asks-check"
		}
		o.args = o.args[1:]
	}
	store, err := planning.Open(dir)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v.\n", err)
		return 1
	}
	repo := store.Layout == planning.LayoutRepo
	if o.fetch || (repo && !o.offline && o.ref == "" && !o.worktree && cmd != "check") {
		if err := store.Git.Fetch(); err != nil {
			if o.fetch {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return 1
			}
			fmt.Fprintf(stderr, "warning: %v; reading what this clone has\n", err)
		}
	}
	worktree := o.worktree || (repo && cmd == "check" && o.ref == "")
	view, err := store.NewView(o.ref, worktree)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	switch cmd {
	case "status", "ready":
		if len(o.args) != 1 {
			fmt.Fprintln(stderr, "Error: name exactly one plan.")
			return 2
		}
		return status(view, o, cmd, stdout, stderr)
	case "check":
		if len(o.args) > 1 {
			fmt.Fprintln(stderr, "Error: name at most one plan.")
			return 2
		}
		return check(view, o, stdout, stderr)
	case "asks", "asks-check":
		if len(o.args) > 0 {
			fmt.Fprintf(stderr, "Error: unexpected argument %q.\n", o.args[0])
			return 2
		}
		if cmd == "asks-check" {
			return asksCheck(view, o, stdout, stderr)
		}
		return asks(view, o, stdout, stderr)
	case "projects":
		return projects(view, o, stdout)
	}
	fmt.Fprintf(stderr, "Error: unknown command %q.\n\n%s", cmd, usage)
	return 2
}

func docsFor(view *planning.View) (*planning.Docs, []*planning.Doc, error) {
	found, err := view.AllDocs("")
	if err != nil {
		return nil, nil, err
	}
	return planning.IndexDocs(found), found, nil
}

func loadPlan(view *planning.View, arg string) (*planning.Plan, error) {
	p, project, err := view.ResolvePlan(arg)
	if err != nil {
		return nil, err
	}
	return view.LoadPlan(p, project)
}

func status(view *planning.View, o opts, cmd string, stdout, stderr io.Writer) int {
	docs, _, err := docsFor(view)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	pl, err := loadPlan(view, o.args[0])
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	repo := view.Store.Layout == planning.LayoutRepo
	threads := pl.Evaluate(docs, view.Store.Git.RemoteBranches(), repo)
	gates := pl.PendingGates(threads)
	if cmd == "ready" {
		for _, t := range threads {
			if t.State == "ready" && t.Doc != nil {
				fmt.Fprintln(stdout, *t.Doc)
			}
		}
		return 0
	}
	if o.json {
		if err := planning.WriteJSON(stdout, planning.StatusJSON(pl, threads, gates, view.Ref, view.Commit, repo)); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}
	planning.PrintStatus(stdout, pl, threads, gates, view.Ref, view.Commit, repo)
	return 0
}

func report(stdout io.Writer, errors, warnings []string) {
	for _, w := range warnings {
		fmt.Fprintf(stdout, "warning: %s\n", w)
	}
	for _, e := range errors {
		fmt.Fprintf(stdout, "error: %s\n", e)
	}
}

func home(view *planning.View) string {
	if view.Store.Layout == planning.LayoutLegacy {
		return view.Projects[0].Name
	}
	return ""
}

func check(view *planning.View, o opts, stdout, stderr io.Writer) int {
	docs, found, err := docsFor(view)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if len(o.args) == 0 {
		errors, warnings := planning.CheckIDs(found, home(view))
		if view.Store.Layout == planning.LayoutRepo {
			errors = append(errors, planning.CheckProjects(view.Projects)...)
		}
		report(stdout, errors, warnings)
		if len(errors) == 0 {
			fmt.Fprintf(stdout, "%d dispatch docs, no shared thread ids.\n", len(found))
			return 0
		}
		return 1
	}
	pl, err := loadPlan(view, o.args[0])
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	errors, warnings := pl.Check(docs, view.Store.Layout)
	idErrors, idWarnings := planning.CheckIDs(found, home(view))
	errors = append(errors, idErrors...)
	warnings = append(warnings, idWarnings...)
	report(stdout, errors, warnings)
	if len(errors) == 0 {
		fmt.Fprintf(stdout, "Plan %s: %d threads, %d gates, no errors.\n", pl.Slug, len(pl.Threads), pl.Gates.Len())
		return 0
	}
	return 1
}

func today(o opts) (time.Time, error) {
	if o.today != "" {
		d, err := time.Parse("2006-01-02", o.today)
		if err != nil {
			return d, fmt.Errorf("--today takes YYYY-MM-DD")
		}
		return d, nil
	}
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
}

func asks(view *planning.View, o opts, stdout, stderr io.Writer) int {
	t, err := today(o)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	var sources []planning.RFCSource
	var notes []string
	if view.Store.Layout == planning.LayoutRepo {
		sources, notes = view.RepoCheckouts(o.project)
	} else {
		sources = []planning.RFCSource{view.LocalRFCSource()}
	}
	res, err := view.Gather(t, o.all, sources, o.project)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	res.Notes = append(notes, res.Notes...)
	if o.json {
		if err := planning.WriteJSON(stdout, planning.AsksJSON(res, view.Ref, view.Commit, t)); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}
	planning.PrintAsks(stdout, res, view.Ref, view.Commit, t, view.Store.Git.Shallow())
	return 0
}

func asksCheck(view *planning.View, o opts, stdout, stderr io.Writer) int {
	errors, warnings, err := view.CheckAsks(o.project)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	report(stdout, errors, warnings)
	if len(errors) == 0 {
		fmt.Fprintln(stdout, "Asks: no errors.")
		return 0
	}
	return 1
}

func projects(view *planning.View, o opts, stdout io.Writer) int {
	if o.json {
		planning.WriteJSON(stdout, view.Projects)
		return 0
	}
	for _, p := range view.Projects {
		fmt.Fprintf(stdout, "%s  %s\n", p.Name, p.Title)
		if len(p.Owns) > 0 {
			fmt.Fprintf(stdout, "  owns: %s\n", strings.Join(p.Owns, ", "))
		}
		var deps []string
		for _, d := range p.DependsOn {
			deps = append(deps, d.Repo)
		}
		if len(deps) > 0 {
			fmt.Fprintf(stdout, "  depends on: %s\n", strings.Join(deps, ", "))
		}
	}
	return 0
}
