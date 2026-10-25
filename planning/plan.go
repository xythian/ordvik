package planning

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	refForm     = regexp.MustCompile(`^(?:gate:[A-Za-z0-9._-]+|[0-9A-Za-z._-]+(?::\d+)?)$`)
	refFormRepo = regexp.MustCompile(`^(?:gate:[A-Za-z0-9._-]+|(?:[a-z0-9][a-z0-9-]*/)?[0-9A-Za-z._-]+(?::\d+)?)$`)
	planStatus  = []string{"draft", "active", "done", "abandoned"}
)

// Plan is a dispatch plan: its dispatches and what each waits on.
type Plan struct {
	Path      string
	Project   string
	Slug      string
	Title     any
	Status    string
	Conductor any
	Threads   []*PlanThread
	Gates     *Gates
}

// PlanThread is one entry of a plan's list: a dispatch, or a sketch of one.
type PlanThread struct {
	ID    string
	After []string
	Raw   map[string]any
}

// Key is the entry's qualified id.
func (t *PlanThread) key(project string) string { return Qualify(project, t.ID) }

// Gates are a plan's gates by id, in the order written.
type Gates struct {
	keys []string
	byID map[string]map[string]any
}

func (g *Gates) Get(id string) (map[string]any, bool) {
	m, ok := g.byID[id]
	return m, ok
}

func (g *Gates) Len() int { return len(g.byID) }

// ResolvePlan finds a plan: a path, <project>/<slug>, or a slug.
func (v *View) ResolvePlan(arg string) (string, *Project, error) {
	if st, err := os.Stat(arg); err == nil && !st.IsDir() {
		abs, _ := filepath.Abs(arg)
		rel, err := filepath.Rel(v.Store.Root, abs)
		if err == nil && !strings.HasPrefix(rel, "..") {
			rel = filepath.ToSlash(rel)
			if p := v.ProjectOf(rel); p != nil {
				return rel, p, nil
			}
		}
		if len(v.Projects) == 1 {
			return abs, v.Projects[0], nil
		}
		return "", nil, fmt.Errorf("%s is not in a project's plans/", arg)
	}
	name := arg
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	var candidates []*Project
	if proj, slug, ok := strings.Cut(name, "/"); ok && v.Store.Layout == LayoutRepo {
		if p := v.Project(proj); p != nil {
			candidates, name = []*Project{p}, slug
		}
	}
	if candidates == nil {
		candidates = v.Projects
	}
	var hits []string
	var hitProject *Project
	for _, p := range candidates {
		for _, d := range []string{p.PlansDir(), p.PlansDir() + "/archive"} {
			rel := d + "/" + name
			if _, ok, _ := v.Files.Read(rel); ok {
				hits = append(hits, rel)
				hitProject = p
				break
			}
		}
	}
	switch len(hits) {
	case 1:
		if _, ok := v.Files.(FSTree); ok {
			return filepath.Join(v.Store.Root, hits[0]), hitProject, nil
		}
		return hits[0], hitProject, nil
	case 0:
		dir := "docs/planning/plans"
		if v.Store.Layout == LayoutRepo {
			dir = "projects/<project>/plans"
		}
		return "", nil, fmt.Errorf("no plan %s (a path, or a slug under %s/)", arg, dir)
	}
	return "", nil, fmt.Errorf("plan %s is in several projects (%s); name it as <project>/<slug>",
		arg, strings.Join(hits, ", "))
}

// LoadPlan reads a plan. p is a path in v.Files, or an absolute path.
func (v *View) LoadPlan(p string, project *Project) (*Plan, error) {
	var text string
	if filepath.IsAbs(p) {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		text = string(b)
	} else {
		t, ok, err := v.Files.Read(p)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("no plan %s", p)
		}
		text = t
	}
	return parsePlan(v.Store.Layout, p, project.Name, text)
}

func parsePlan(layout, p, project, text string) (*Plan, error) {
	meta, _, err := SplitFrontmatter(text)
	if err != nil {
		return nil, fmt.Errorf("%s: frontmatter is not valid YAML (%s)", p, yamlErrorText(err))
	}
	if len(meta) == 0 {
		return nil, fmt.Errorf("%s has no YAML frontmatter.", p)
	}
	pl := &Plan{Path: p, Project: project, Gates: &Gates{byID: map[string]map[string]any{}}}
	list := get(meta, "threads")
	if layout == LayoutRepo {
		if d, ok := meta["dispatches"]; ok {
			list = d
		}
	}
	for _, e := range asList(orDefault(list, nil)) {
		raw, _ := e.(map[string]any)
		if raw == nil {
			raw = map[string]any{}
		}
		t := &PlanThread{ID: strings.TrimSpace(pyStr(getOr(raw, "id", ""))), Raw: raw}
		for _, a := range asList(orDefault(get(raw, "after"), nil)) {
			t.After = append(t.After, pyStr(a))
		}
		pl.Threads = append(pl.Threads, t)
	}
	for _, e := range asList(orDefault(get(meta, "gates"), nil)) {
		raw, _ := e.(map[string]any)
		if raw == nil {
			raw = map[string]any{}
		}
		id := pyStr(get(raw, "id"))
		if _, ok := pl.Gates.byID[id]; !ok {
			pl.Gates.keys = append(pl.Gates.keys, id)
		}
		pl.Gates.byID[id] = raw
	}
	base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	pl.Slug = pyStr(orDefault(get(meta, "plan"), base))
	pl.Title = get(meta, "title")
	pl.Status = strings.ToLower(pyStr(orDefault(get(meta, "status"), "active")))
	pl.Conductor = get(meta, "conductor")
	return pl, nil
}

// condition is (satisfied, description) for one `after:` entry.
func (pl *Plan) condition(ref string, docs *Docs) (bool, string) {
	if strings.HasPrefix(ref, "gate:") {
		g, ok := pl.Gates.Get(ref[5:])
		if !ok {
			return false, ref + " (no such gate)"
		}
		return truthy(get(g, "passed")), "gate " + ref[5:]
	}
	thread, task, _ := strings.Cut(ref, ":")
	doc := docs.Get(Qualify(pl.Project, thread))
	if doc == nil {
		return false, ref + " (no dispatch doc yet)"
	}
	if doc.Status == "done" {
		return true, ref
	}
	if task != "" {
		n, err := strconv.Atoi(task)
		return err == nil && doc.Tasks[n], fmt.Sprintf("%s Task %s", thread, task)
	}
	return false, thread
}

// Milestone is what other threads wait on in one thread: "Task N" or
// "done", and the ids that wait on it.
type Milestone struct {
	Name string
	IDs  []string
}

func (pl *Plan) milestones(thread string) []Milestone {
	var out []Milestone
	index := map[string]int{}
	key := Qualify(pl.Project, thread)
	for _, t := range pl.Threads {
		for _, a := range t.After {
			dep, task, _ := strings.Cut(a, ":")
			if strings.HasPrefix(a, "gate:") || Qualify(pl.Project, dep) != key {
				continue
			}
			name := "done"
			if task != "" {
				name = "Task " + task
			}
			i, ok := index[name]
			if !ok {
				i = len(out)
				index[name] = i
				out = append(out, Milestone{Name: name})
			}
			out[i].IDs = append(out[i].IDs, t.ID)
		}
	}
	return out
}

func (pl *Plan) threadDeps() (map[string]map[string]bool, []string) {
	deps := map[string]map[string]bool{}
	var order []string
	for _, t := range pl.Threads {
		k := t.key(pl.Project)
		if _, ok := deps[k]; !ok {
			order = append(order, k)
		}
		set := map[string]bool{}
		for _, a := range t.After {
			if strings.HasPrefix(a, "gate:") {
				continue
			}
			dep, _, _ := strings.Cut(a, ":")
			set[Qualify(pl.Project, dep)] = true
		}
		deps[k] = set
	}
	return deps, order
}

// Stages: 1 for a thread that waits on no other thread, else one more
// than the latest stage it waits on. Gates add no stage.
func (pl *Plan) Stages() map[string]int {
	deps, _ := pl.threadDeps()
	memo := map[string]int{}
	var stage func(n string, seen map[string]bool) int
	stage = func(n string, seen map[string]bool) int {
		if s, ok := memo[n]; ok {
			return s
		}
		if seen[n] { // a cycle; check reports it
			return 1
		}
		next := map[string]bool{n: true}
		for k := range seen {
			next[k] = true
		}
		best := 0
		for d := range deps[n] {
			if _, in := deps[d]; in {
				if s := stage(d, next); s > best {
					best = s
				}
			}
		}
		memo[n] = 1 + best
		return memo[n]
	}
	out := map[string]int{}
	for n := range deps {
		out[n] = stage(n, map[string]bool{})
	}
	return out
}

// ThreadState is one plan thread evaluated against the dispatch docs.
type ThreadState struct {
	ID         string            `json:"id"`
	Project    string            `json:"project,omitempty"`
	Stage      int               `json:"stage"`
	Title      string            `json:"title"`
	Doc        *string           `json:"doc"`
	Repo       string            `json:"repo,omitempty"`
	Branch     string            `json:"branch,omitempty"`
	After      []string          `json:"after"`
	WaitingOn  []string          `json:"waiting_on"`
	TasksDone  int               `json:"tasks_done"`
	Tasks      int               `json:"tasks"`
	State      string            `json:"state"`
	Milestones orderedMilestones `json:"milestones"`
	Launched   *string           `json:"launched,omitempty"`
}

type orderedMilestones []Milestone

func (m orderedMilestones) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.WriteString("{")
	for i, ms := range m {
		if i > 0 {
			b.WriteString(",")
		}
		k, _ := marshalJSON(ms.Name)
		v, _ := marshalJSON(ms.IDs)
		b.Write(k)
		b.WriteString(":")
		b.Write(v)
	}
	b.WriteString("}")
	return []byte(b.String()), nil
}

// Evaluate gives each plan thread its state and what it waits on.
func (pl *Plan) Evaluate(docs *Docs, branches map[string]bool, repoLayout bool) []*ThreadState {
	stages := pl.Stages()
	var out []*ThreadState
	for _, t := range pl.Threads {
		doc := docs.Get(t.key(pl.Project))
		waiting := []string{}
		for _, a := range t.After {
			if ok, desc := pl.condition(a, docs); !ok {
				waiting = append(waiting, desc)
			}
		}
		e := &ThreadState{ID: t.ID, Stage: stages[t.key(pl.Project)], After: t.After, WaitingOn: waiting}
		if e.After == nil {
			e.After = []string{}
		}
		if repoLayout {
			e.Project = pl.Project
		}
		if doc != nil && doc.Title != nil && *doc.Title != "" {
			e.Title = *doc.Title
		} else if title := get(t.Raw, "title"); truthy(title) {
			e.Title = pyStr(title)
		}
		if doc != nil {
			p := doc.Path
			e.Doc = &p
			e.TasksDone, e.Tasks = doc.TasksDone(), len(doc.Tasks)
			if repoLayout {
				e.Repo, e.Branch = doc.Repo, doc.BranchName()
			}
		}
		launched := get(t.Raw, "launched")
		switch {
		case truthy(get(t.Raw, "dropped")):
			e.State = "dropped"
		case doc == nil && truthy(launched):
			e.State = "started" // launched from a doc not yet on the ref
		case doc == nil && len(waiting) == 0:
			e.State = "write-doc"
		case doc == nil:
			e.State = "planned"
		case doc.Status == "done":
			e.State = "done"
		case doc.Status == "superseded":
			e.State = "superseded"
		case len(waiting) > 0:
			e.State = "blocked"
		case doc.Executor == "human":
			e.State = "human"
		case truthy(launched) || (doc.Branch != nil && branches[doc.BranchName()]) || e.TasksDone > 0:
			e.State = "started"
		default:
			e.State = "ready"
		}
		e.Milestones = pl.milestones(t.ID)
		if e.Milestones == nil {
			e.Milestones = orderedMilestones{}
		}
		if truthy(launched) {
			s := pyStr(launched)
			e.Launched = &s
		}
		out = append(out, e)
	}
	return out
}

// PendingGate is a gate not yet passed that an unfinished thread waits on.
type PendingGate struct {
	ID       string   `json:"id"`
	Ask      any      `json:"ask"`
	Unblocks []string `json:"unblocks"`
}

func (pl *Plan) PendingGates(threads []*ThreadState) []PendingGate {
	var out []PendingGate
	index := map[string]int{}
	for _, t := range threads {
		if t.State == "done" || t.State == "superseded" || t.State == "dropped" {
			continue
		}
		for _, a := range t.After {
			if !strings.HasPrefix(a, "gate:") {
				continue
			}
			g, ok := pl.Gates.Get(a[5:])
			if !ok || truthy(get(g, "passed")) {
				continue
			}
			i, seen := index[a[5:]]
			if !seen {
				i = len(out)
				index[a[5:]] = i
				out = append(out, PendingGate{ID: a[5:], Ask: getOr(g, "ask", "")})
			}
			out[i].Unblocks = append(out[i].Unblocks, t.ID)
		}
	}
	if out == nil {
		out = []PendingGate{}
	}
	return out
}

// Check is (errors, warnings) for the plan against the dispatch docs.
func (pl *Plan) Check(docs *Docs, layout string) (errors, warnings []string) {
	form := refForm
	if layout == LayoutRepo {
		form = refFormRepo
	}
	if !contains(planStatus, pl.Status) {
		errors = append(errors, fmt.Sprintf("status %s is not one of %s", pyStrRepr(pl.Status), strings.Join(planStatus, ", ")))
	}
	count := map[string]int{}
	inPlan := map[string]bool{}
	for _, t := range pl.Threads {
		count[t.ID]++
		inPlan[t.key(pl.Project)] = true
	}
	var dups []string
	for id, n := range count {
		if n > 1 {
			dups = append(dups, id)
		}
	}
	sort.Strings(dups)
	for _, id := range dups {
		errors = append(errors, fmt.Sprintf("thread %s is listed more than once", id))
	}
	for _, t := range pl.Threads {
		if t.ID == "" {
			errors = append(errors, "a thread entry has no id")
			continue
		}
		if get(t.Raw, "wave") != nil {
			warnings = append(warnings, fmt.Sprintf("thread %s: plans have no waves (stages come from after); drop wave", t.ID))
		}
		doc := docs.Get(t.key(pl.Project))
		if doc == nil && !truthy(get(t.Raw, "title")) {
			errors = append(errors, fmt.Sprintf("thread %s has no dispatch doc yet and no title", t.ID))
		}
		for _, a := range t.After {
			if !form.MatchString(a) {
				errors = append(errors, fmt.Sprintf("thread %s: after %s is not THREAD, THREAD:TASK or gate:ID", t.ID, pyStrRepr(a)))
				continue
			}
			if strings.HasPrefix(a, "gate:") {
				if _, ok := pl.Gates.Get(a[5:]); !ok {
					errors = append(errors, fmt.Sprintf("thread %s: after %s names no gate in the plan", t.ID, a))
				}
				continue
			}
			dep, task, _ := strings.Cut(a, ":")
			depKey := Qualify(pl.Project, dep)
			depDoc := docs.Get(depKey)
			if !inPlan[depKey] {
				if strings.Contains(dep, "/") && depDoc != nil {
					// another project's dispatch: a wait across projects
				} else if strings.Contains(dep, "/") {
					errors = append(errors, fmt.Sprintf("thread %s: after %s names no dispatch doc", t.ID, a))
					continue
				} else {
					errors = append(errors, fmt.Sprintf("thread %s: after %s names a thread not in the plan", t.ID, a))
					continue
				}
			}
			if task != "" && depDoc != nil {
				n, _ := strconv.Atoi(task)
				if _, ok := depDoc.Tasks[n]; !ok {
					errors = append(errors, fmt.Sprintf("thread %s: after %s, but %s has no Task %s", t.ID, a, depDoc.Path, task))
				}
			}
		}
		if doc == nil {
			continue
		}
		if doc.YAMLError != "" {
			errors = append(errors, fmt.Sprintf("%s: frontmatter is not valid YAML (%s); wt launch cannot read it", doc.Path, doc.YAMLError))
		}
		if !pyEqual(doc.Plan, pl.Slug) {
			warnings = append(warnings, fmt.Sprintf("%s: frontmatter plan is %s, expected %s", doc.Path, pyRepr(doc.Plan), pyStrRepr(pl.Slug)))
		}
		if truthy(doc.Wave) {
			warnings = append(warnings, fmt.Sprintf("%s: has wave %s; plans have no waves, drop it", doc.Path, pyStr(doc.Wave)))
		}
		if len(doc.DependsOn) > 0 {
			planned := map[string]bool{}
			for _, a := range t.After {
				if !strings.HasPrefix(a, "gate:") {
					dep, _, _ := strings.Cut(a, ":")
					planned[dep] = true
				}
			}
			have := map[string]bool{}
			for _, d := range doc.DependsOn {
				have[d] = true
			}
			if !sameSet(planned, have) {
				warnings = append(warnings, fmt.Sprintf("%s: depends_on %s differs from the plan's after %s; the plan wins, drop depends_on from the doc",
					doc.Path, pyRepr(doc.DependsOn), pyRepr(t.After)))
			}
		}
	}
	for _, doc := range docs.All() {
		if doc.Project == pl.Project && pyEqual(doc.Plan, pl.Slug) && !inPlan[doc.Key()] {
			errors = append(errors, fmt.Sprintf("%s says plan %s but the plan does not list thread %s", doc.Path, pl.Slug, doc.ID))
		}
	}
	errors = append(errors, pl.cycles()...)
	return errors, warnings
}

func (pl *Plan) cycles() []string {
	deps, order := pl.threadDeps()
	state := map[string]string{}
	var errors []string
	var visit func(n string, stack []string)
	visit = func(n string, stack []string) {
		state[n] = "visiting"
		var next []string
		for m := range deps[n] {
			next = append(next, m)
		}
		sort.Strings(next)
		for _, m := range next {
			if state[m] == "visiting" {
				path := append(append(append([]string{}, stack...), n), m)
				for i := range path {
					path[i] = Unqualify(pl.Project, path[i])
				}
				errors = append(errors, "cycle: "+strings.Join(path, " -> "))
			} else if _, in := deps[m]; in && state[m] == "" {
				visit(m, append(append([]string{}, stack...), n))
			}
		}
		state[n] = "done"
	}
	for _, n := range order {
		if state[n] == "" {
			visit(n, nil)
		}
	}
	return errors
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
