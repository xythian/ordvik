// Command ordvik reads and checks planning data: dispatch docs, plans and
// operator asks, in a code repository's docs/planning/ or in the planning
// repository.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
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
  ordvik planning link <project>/<id> --pr <owner/name#n> [--task <n>]... [--done]
                                           Record a PR on a dispatch doc and push the planning repository
  ordvik planning ask new --project <p> --question <q> --option "label=then"... --recommended <n>
                          [--context <text>] [--kind decide|review|approve|do] [--category <c>]...
                          [--from-thread <t>] [--from-pr <owner/name#n>] [--from-dispatch <p>/<id>]
                          [--gate <p>/<dispatch>:<task> | --gate plan:<p>/<plan>:<gate>]
                                           Ask the operator: write the ask (2 to 4 options, one
                                           recommended, index from 0) and push the planning repository
  ordvik planning ask show <id> [--json]   The ask and its answer
  ordvik planning ask wait <id> [--timeout 15m] [--interval 30s]
                                           Fetch until the ask is answered (exit 0) or the timeout passes (exit 3)
  ordvik planning ask answer <id> <n|other> [--text <words>] [--by <name>] [--outcome <o>]
                                           Record the operator's answer and push it
  ordvik planning ask defer <id> <YYYY-MM-DD>
                                           Take an ask file off the queue until that date and push it

A dispatch's tasks are done once a PR linked to them (prs:) has merged, and
the dispatch once a PR linked with --done has merged and no human gate is
open. PR state comes from the GitHub API ($GITHUB_TOKEN or $GH_TOKEN, else
gh); --no-prs reads only the marks in the docs.

Every command takes:
  --ref <ref>   Read dispatch docs from <ref> (default origin/HEAD, else origin/main or master)
  --fetch       git fetch origin first
  --offline     In the planning repository, do not fetch (it fetches by default)
  --worktree    Read everything from the working tree (the default for check
                in the planning repository, so an edit is checked before it is committed)
  --no-prs      Do not read linked PRs' state

<plan> is a path, <project>/<slug>, or a slug under a project's plans/.

In a code repository, docs/planning/ is read as one project, the way
scripts/plan and scripts/asks read it: dispatch docs from the ref, plans
and ask files from the working tree.

ordvik plan ... and ordvik asks ... are scripts/plan and scripts/asks.

An ask id is what asks --json prints: ask:<project>/<slug> (or <project>/<slug>,
or the bare slug), dispatch:<project>/<id>:<task> (or <project>/<id>:<task>),
plan:<project>/<plan>:<gate>.

Exit status: 0, 1 on errors (check finds any, or the store cannot be read), 2 on bad usage,
3 when ask wait times out.
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
	noPRs    bool
	pr       string
	tasks    []int
	done     bool
	args     []string

	// ask
	question    string
	options     []string
	recommended string
	context     string
	kind        string
	categories  []string
	fromThread  string
	fromPR      string
	fromDisp    string
	gate        string
	timeout     time.Duration
	interval    time.Duration
	text        string
	by          string
	outcome     string
}

func parse(argv []string) (opts, error) {
	o := opts{timeout: 15 * time.Minute, interval: 30 * time.Second}
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
		case "--pr":
			o.pr, err = takes()
		case "--task":
			var t string
			if t, err = takes(); err == nil {
				var n int
				if n, err = strconv.Atoi(t); err != nil || n < 1 {
					err = fmt.Errorf("--task takes a task number")
				}
				o.tasks = append(o.tasks, n)
			}
		case "--question":
			o.question, err = takes()
		case "--option":
			var v string
			if v, err = takes(); err == nil {
				o.options = append(o.options, v)
			}
		case "--recommended":
			o.recommended, err = takes()
		case "--context":
			o.context, err = takes()
		case "--kind":
			o.kind, err = takes()
		case "--category":
			var v string
			if v, err = takes(); err == nil {
				for _, c := range strings.Split(v, ",") {
					if c = strings.TrimSpace(c); c != "" {
						o.categories = append(o.categories, c)
					}
				}
			}
		case "--from-thread":
			o.fromThread, err = takes()
		case "--from-pr":
			o.fromPR, err = takes()
		case "--from-dispatch":
			o.fromDisp, err = takes()
		case "--gate":
			o.gate, err = takes()
		case "--timeout", "--interval":
			var v string
			if v, err = takes(); err == nil {
				var d time.Duration
				if d, err = time.ParseDuration(v); err != nil || d < 0 {
					err = fmt.Errorf("%s takes a duration, such as 30s or 15m", a)
				} else if a == "--timeout" {
					o.timeout = d
				} else {
					o.interval = d
				}
			}
		case "--text":
			o.text, err = takes()
		case "--by":
			o.by, err = takes()
		case "--outcome":
			o.outcome, err = takes()
		case "--done":
			o.done = true
		case "--no-prs":
			o.noPRs = true
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
	if cmd == "link" {
		return link(store, o, stdout, stderr)
	}
	if cmd == "ask" {
		return ask(store, o, stdout, stderr)
	}
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

// prSource reads linked PRs' state; tests replace it.
var prSource planning.PRSource = &planning.GitHub{}

func docsFor(view *planning.View) (*planning.Docs, []*planning.Doc, error) {
	found, err := view.AllDocs("")
	if err != nil {
		return nil, nil, err
	}
	return planning.IndexDocs(found), found, nil
}

// derivedDocs is docsFor with done derived from merged PRs (unless
// --no-prs), its notes on stderr.
func derivedDocs(view *planning.View, o opts, stderr io.Writer) (*planning.Docs, []*planning.Doc, error) {
	docs, found, err := docsFor(view)
	if err == nil && !o.noPRs {
		for _, n := range planning.DerivePRs(found, prSource) {
			fmt.Fprintf(stderr, "note: %s\n", n)
		}
	}
	return docs, found, err
}

func loadPlan(view *planning.View, arg string) (*planning.Plan, error) {
	p, project, err := view.ResolvePlan(arg)
	if err != nil {
		return nil, err
	}
	return view.LoadPlan(p, project)
}

func status(view *planning.View, o opts, cmd string, stdout, stderr io.Writer) int {
	docs, _, err := derivedDocs(view, o, stderr)
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
		errors = append(errors, planning.CheckPRs(found)...)
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
	if !o.noPRs {
		view.PRs = prSource
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
			var owns []string
			for _, r := range p.Owns {
				if p.IsPublic(r) {
					r += " (public)"
				}
				owns = append(owns, r)
			}
			fmt.Fprintf(stdout, "  owns: %s\n", strings.Join(owns, ", "))
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

func link(store *planning.Store, o opts, stdout, stderr io.Writer) int {
	if len(o.args) != 1 || o.pr == "" {
		fmt.Fprintln(stderr, "Error: link takes <project>/<id> and --pr <owner/name#n>.")
		return 2
	}
	repo, n, ok := planning.ParsePRRef(o.pr)
	if !ok {
		fmt.Fprintf(stderr, "Error: --pr %q is not owner/name#number.\n", o.pr)
		return 2
	}
	l := planning.PRLink{Repo: repo, Number: n, Tasks: o.tasks, Done: o.done}
	path, added, err := store.Link(o.args[0], l)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if !added {
		fmt.Fprintf(stdout, "%s already links %s.\n", path, l.Ref())
		return 0
	}
	fmt.Fprintf(stdout, "Linked %s on %s and pushed.\n", l.Ref(), path)
	return 0
}
