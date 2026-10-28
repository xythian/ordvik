package planning

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// An ask is something only the operator can move: a decision, a review,
// or a step that needs their accounts, devices or approval. Most asks are
// already recorded in planning files, so they are read rather than kept a
// second time:
//
//	dispatch  an open human_gates entry of an active dispatch doc
//	plan      a gate of an active plan that is not passed and that an
//	          unfinished thread waits on
//	rfc       an RFC at status `considering`
//	ask       a file in a project's asks/, for asks with no other home

var (
	askKinds    = []string{"decide", "review", "approve", "do"}
	askOutcomes = []string{"done", "decided", "rejected", "automate", "dropped"}
	gateTask    = regexp.MustCompile(`(?i)^\s*Task\s+([A-Z]?\d+)\b\W*(.*)$`)
	taskLine    = regexp.MustCompile(`(?i)^#{2,4}\s+Task\s+([A-Z]?\d+)\b(.*)$`)
	rfcPath     = regexp.MustCompile(`(?:^|/)docs/rfcs/(\d{4})-[^/]+\.md$`)
	rfcStatus   = regexp.MustCompile("(?im)^\\s*(?:[-*]\\s*)?\\*\\*Status\\*\\*\\s*:\\s*`?([A-Za-z-]+)")
)

const (
	StaleDraftDays = 60
	Recurring      = 3
)

// Inferred from a gate's wording; an ask file states its own. Order
// matters: a step that needs the operator's hands, devices or accounts is
// `do` even when it is part of a rollout; a rollout of existing commands
// is `approve`.
var kindRules = []struct {
	kind string
	rx   *regexp.Regexp
}{
	{"do", regexp.MustCompile(`iphone|ipad|safari|vision pro|\bdevices?\b|by hand|console|1password|\blogin\b|install on|` +
		`\bfiles?\b|\btailnet\b|tagowners|\bgrant\b|restore|drills?\b`)},
	{"decide", regexp.MustCompile(`\b(accept|accepts|confirm|decide|decision|decisions|choose|design|sign off)\b`)},
	{"review", regexp.MustCompile(`\b(review|reviews|inspect|checks?|parity|verify|verifies|validation)\b`)},
	{"approve", regexp.MustCompile(`\b(approve|approves|approval|production|roll ?out|rollout|converge|upgrade|bootstrap|cutover)\b`)},
	{"do", regexp.MustCompile(`\b(run|runs|deploy|create|creates|install|rebuild|provision|clean up|add|adds|enter)\b`)},
}

var categoryRules = []struct {
	name string
	rx   *regexp.Regexp
}{
	{"tailnet-policy", regexp.MustCompile(`\btailnet\b|tagowners|\bacl\b|\bgrant\b|owner-only tag`)},
	{"restore-drill", regexp.MustCompile(`restore|--from-backup`)},
	{"device-check", regexp.MustCompile(`iphone|ipad|safari|vision pro|devices?\b|desktop browser`)},
	{"rfc", regexp.MustCompile(`\brfc\b`)},
	{"credentials", regexp.MustCompile(`1password|\bcas?\b|credentials?\b|enrollment`)},
	{"production-rollout", regexp.MustCompile(`production|roll ?out|rollout|bootstrap|converge|cutover|deploy|upgrade|hypervisor`)},
}

func InferKind(text string) string {
	low := strings.ToLower(text)
	for _, r := range kindRules {
		if r.rx.MatchString(low) {
			return r.kind
		}
	}
	return "do"
}

func InferCategories(text string) []string {
	low := strings.ToLower(text)
	out := []string{}
	for _, r := range categoryRules {
		if r.rx.MatchString(low) {
			out = append(out, r.name)
		}
	}
	return out
}

// Ask is one ask. The JSON keys are scripts/asks'; some are present only
// for some sources, so those are pointers.
type Ask struct {
	ID            string    `json:"id"`
	Project       string    `json:"project,omitempty"`
	Repo          string    `json:"repo,omitempty"`
	Source        string    `json:"source"`
	Kind          string    `json:"kind"`
	Categories    []string  `json:"categories"`
	State         string    `json:"state"`
	DeferredUntil *string   `json:"deferred_until,omitempty"`
	Unmarked      *[]string `json:"unmarked,omitempty"`
	Label         string    `json:"label"`
	Text          any       `json:"text"`
	Where         string    `json:"where"`
	Title         *any      `json:"title,omitempty"`
	From          *any      `json:"from,omitempty"`
	Unblocks      []string  `json:"unblocks"`
	Raised        *string   `json:"raised"`
	AgeDays       *int      `json:"age_days"`

	// The ask's shape (askshape.go), when it has one.
	Options     []AskOption `json:"options,omitempty"`
	Recommended *int        `json:"recommended,omitempty"`
	Context     string      `json:"context,omitempty"`
	Answered    *string     `json:"answered,omitempty"`
	Answer      any         `json:"answer,omitempty"`
	AnswerText  string      `json:"answer_text,omitempty"`
	AnsweredBy  string      `json:"answered_by,omitempty"`

	hasDeferred bool
}

func (a *Ask) unmarkedCount() int {
	if a.Unmarked == nil {
		return 0
	}
	return len(*a.Unmarked)
}

// Drafts counts RFC drafts and lists those untouched for StaleDraftDays.
type Drafts struct {
	Count      int          `json:"count"`
	Stale      []StaleDraft `json:"stale"`
	StaleCount int          `json:"stale_count"`
}

type StaleDraft struct {
	Path     string `json:"path"`
	Repo     string `json:"repo,omitempty"`
	Title    string `json:"title"`
	IdleDays *int   `json:"idle_days"`
}

// AsksResult is everything `asks status` reports.
type AsksResult struct {
	Asks      []*Ask
	Recurring []Milestone // category and the ids of its open `do` asks
	Drafts    Drafts
	Notes     []string
}

type gateEntry struct {
	doc       *Doc
	task      string // "" when the entry names no task
	text      string
	fields    map[string]any // the entry's mapping, when it is one
	marks     map[string]bool
	gateTasks map[string]bool
}

func taskMarks(body string) map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		if m := taskLine.FindStringSubmatch(line); m != nil {
			out[strings.ToUpper(m[1])] = doneMark.MatchString(m[2])
		}
	}
	return out
}

type taskKey struct {
	group  int
	n      int
	letter string
}

func taskOrder(id string) taskKey {
	if n, err := strconv.Atoi(id); err == nil {
		return taskKey{0, n, ""}
	}
	n, _ := strconv.Atoi(id[1:])
	return taskKey{1, n, id[:1]}
}

func (a taskKey) less(b taskKey) bool {
	if a.group != b.group {
		return a.group < b.group
	}
	if a.n != b.n {
		return a.n < b.n
	}
	return a.letter < b.letter
}

func (v *View) dispatchGates(docs *Docs) []gateEntry {
	var gates []gateEntry
	for _, doc := range docs.All() {
		if doc.Archived {
			continue
		}
		meta, body := doc.Meta, doc.Body
		var entries []any
		if doc.YAMLError == "" {
			entries = asList(get(meta, "human_gates"))
		}
		marks := taskMarks(body)
		gateTasks := map[string]bool{}
		type parsed struct {
			task, text string
			fields     map[string]any
		}
		var ps []parsed
		for _, e := range entries {
			g, fields := gateFields(e)
			m := gateTask.FindStringSubmatch(g)
			task, text := "", g
			if m != nil {
				task, text = strings.ToUpper(m[1]), m[2]
				gateTasks[task] = true
			}
			ps = append(ps, parsed{task, strings.TrimSpace(text), fields})
		}
		for _, p := range ps {
			gates = append(gates, gateEntry{doc: doc, task: p.task, text: p.text, fields: p.fields, marks: marks, gateTasks: gateTasks})
		}
	}
	return gates
}

// ActivePlans is every plan with status active, of every project.
func (v *View) ActivePlans() ([]*Plan, error) {
	var out []*Plan
	for _, p := range v.Projects {
		dir := p.PlansDir()
		var names []string
		if fs, ok := v.Files.(FSTree); ok {
			names = listDir(fs.Root, dir)
		} else {
			files, err := v.Files.Files(dir)
			if err != nil {
				return nil, err
			}
			for _, f := range files {
				if rel := f[len(dir)+1:]; !strings.Contains(rel, "/") {
					names = append(names, rel)
				}
			}
		}
		for _, name := range names {
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			pl, err := v.LoadPlan(dir+"/"+name, p)
			if err != nil {
				return nil, err
			}
			if pl.Status == "active" {
				out = append(out, pl)
			}
		}
	}
	return out, nil
}

func planWaiters(plans []*Plan, docs *Docs, threadKey, task string) []string {
	set := map[string]bool{}
	for _, p := range plans {
		for _, t := range p.Threads {
			if truthy(get(t.Raw, "dropped")) {
				continue
			}
			if d := docs.Get(t.key(p.Project)); d != nil && (d.Status == "done" || d.Status == "superseded") {
				continue
			}
			for _, a := range t.After {
				dep, n, _ := strings.Cut(a, ":")
				if Qualify(p.Project, dep) == threadKey && (n == "" || task == "" || strings.ToUpper(n) == task) {
					set[t.ID] = true
					break
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// RFCSource is a repository whose RFCs count as asks: the store itself in
// the legacy layout, a checkout of an owned repository in the planning
// repository.
type RFCSource struct {
	Repo    string // owner/name; "" for the store's own repository
	Project string
	Git     Git
	Ref     string
}

// Gather lists every open ask. today is the date ages count to.
func (v *View) Gather(today time.Time, showAll bool, rfcs []RFCSource, only string) (*AsksResult, error) {
	return v.gather(today, showAll, rfcs, only, false)
}

// gather is Gather, with answered asks too when answered is set.
func (v *View) gather(today time.Time, showAll bool, rfcs []RFCSource, only string, answered bool) (*AsksResult, error) {
	found, err := v.AllDocs(only)
	if err != nil {
		return nil, err
	}
	var prNotes []string
	if v.PRs != nil {
		prNotes = DerivePRs(found, v.PRs)
	}
	docs := IndexDocs(found)
	gates := v.dispatchGates(docs)
	plans, err := v.ActivePlans()
	if err != nil {
		return nil, err
	}
	repo := v.Store.Layout == LayoutRepo
	home := ""
	if !repo {
		home = v.Projects[0].Name
	}
	added := map[string]time.Time{}
	for _, p := range v.Projects {
		for k, d := range v.Store.Git.PathDates(v.gitRef(), p.DispatchDir(), true) {
			added[k] = d
		}
	}
	res := &AsksResult{Notes: prNotes}
	for _, g := range gates {
		doc := g.doc
		if doc.Status != "active" && doc.Status != "" {
			continue
		}
		if g.task != "" && g.marks[g.task] {
			continue
		}
		shape := parseShape(g.fields)
		if shape.Answered != "" && !answered {
			continue
		}
		unmarked := []string{}
		if g.task != "" {
			for n, done := range g.marks {
				if taskOrder(n).less(taskOrder(g.task)) && !g.gateTasks[n] && !done {
					unmarked = append(unmarked, n)
				}
			}
			sort.Slice(unmarked, func(i, j int) bool { return taskOrder(unmarked[i]).less(taskOrder(unmarked[j])) })
		}
		thread := Unqualify(home, doc.Key())
		a := &Ask{
			ID:         "dispatch:" + thread,
			Source:     "dispatch",
			Kind:       InferKind(g.text),
			Categories: InferCategories(g.text),
			State:      "ready",
			Unmarked:   &unmarked,
			Label:      thread,
			Text:       g.text,
			Where:      doc.Path,
			Unblocks:   planWaiters(plans, docs, doc.Key(), g.task),
		}
		if repo {
			a.Project = doc.Project
		}
		if g.task != "" {
			a.ID += ":" + g.task
			a.Label += " Task " + g.task
		}
		if k, ok := get(g.fields, "kind").(string); ok && contains(askKinds, k) {
			a.Kind = k
		} else if len(shape.Options) > 0 {
			a.Kind = "decide" // options are a decision, whatever the wording
		}
		if c := get(g.fields, "category"); c != nil {
			a.Categories = asStrings(c)
		}
		if from := get(g.fields, "from"); from != nil {
			a.From = &from
		}
		a.setShape(shape)
		var title any
		if doc.Title != nil {
			title = *doc.Title
		}
		a.Title = &title
		if d, ok := added[doc.Path]; ok {
			s := isoDate(d)
			a.Raised = &s
		}
		res.Asks = append(res.Asks, a)
	}

	for _, p := range plans {
		threads := p.Evaluate(docs, map[string]bool{}, false)
		for _, pg := range p.PendingGates(threads) {
			raw, _ := p.Gates.Get(pg.ID)
			shape := parseShape(raw)
			if shape.Answered != "" && !answered {
				continue
			}
			where := p.Path
			if filepath.IsAbs(where) {
				where, _ = filepath.Rel(v.Store.Root, where)
			}
			slug := p.Slug
			if repo {
				slug = p.Project + "/" + p.Slug
			}
			a := &Ask{
				ID:         "plan:" + slug + ":" + pg.ID,
				Source:     "plan",
				Kind:       "decide",
				Categories: InferCategories(pyStr(pg.Ask)),
				State:      "ready",
				Label:      slug + " gate " + pg.ID,
				Text:       pg.Ask,
				Where:      where,
				Unblocks:   pg.Unblocks,
			}
			if repo {
				a.Project = p.Project
			}
			if from := get(raw, "from"); from != nil {
				a.From = &from
			}
			a.setShape(shape)
			res.Asks = append(res.Asks, a)
		}
	}

	var drafts []StaleDraft
	for _, src := range rfcs {
		asks, ds, err := rfcAsks(src, today)
		if err != nil {
			res.Notes = append(res.Notes, err.Error())
			continue
		}
		if repo {
			for _, a := range asks {
				a.Project = src.Project
			}
		}
		res.Asks = append(res.Asks, asks...)
		drafts = append(drafts, ds...)
	}
	var stale []StaleDraft
	for _, d := range drafts {
		if d.IdleDays != nil && *d.IdleDays >= StaleDraftDays {
			stale = append(stale, d)
		}
	}

	for _, p := range v.Projects {
		if only != "" && p.Name != only {
			continue
		}
		files, _ := v.askFiles(p)
		for _, f := range files {
			if f.answered && !answered {
				continue
			}
			m := f.meta
			state := "ready"
			var deferred *string
			if truthy(get(m, "defer_until")) {
				if d, ok := asDate(get(m, "defer_until")); ok && d.After(today) {
					state = "deferred"
					s := isoDate(d)
					deferred = &s
				}
			}
			label := f.slug
			if repo {
				label = p.Name + "/" + f.slug
			}
			a := &Ask{
				ID:            "ask:" + label,
				Source:        "ask",
				Kind:          pyStr(getOr(m, "kind", "do")),
				Categories:    asStrings(get(m, "category")),
				State:         state,
				DeferredUntil: deferred,
				hasDeferred:   true,
				Label:         label,
				Text:          pyStr(getOr(m, "ask", "")),
				Where:         f.path,
				Unblocks:      asStrings(get(m, "unblocks")),
			}
			from := get(m, "from")
			a.From = &from
			if r := get(m, "raised"); truthy(r) {
				s := pyStr(r)
				a.Raised = &s
			}
			a.setShape(parseShape(m))
			if repo {
				a.Project = p.Name
			}
			res.Asks = append(res.Asks, a)
		}
	}

	for _, a := range res.Asks {
		if a.Raised != nil {
			if d, ok := parseDate(*a.Raised); ok {
				n := int(today.Sub(d).Hours() / 24)
				a.AgeDays = &n
			}
		}
	}
	index := map[string]int{}
	for _, a := range res.Asks {
		if a.Kind != "do" || a.State == "deferred" {
			continue
		}
		for _, c := range a.Categories {
			i, ok := index[c]
			if !ok {
				i = len(res.Recurring)
				index[c] = i
				res.Recurring = append(res.Recurring, Milestone{Name: c})
			}
			res.Recurring[i].IDs = append(res.Recurring[i].IDs, a.ID)
		}
	}
	res.Drafts = Drafts{Count: len(drafts), Stale: []StaleDraft{}, StaleCount: len(stale)}
	if showAll && stale != nil {
		res.Drafts.Stale = stale
	}
	if res.Asks == nil {
		res.Asks = []*Ask{}
	}
	return res, nil
}

// gitRef is the ref git log reads dates from.
func (v *View) gitRef() string {
	if _, ok := v.Docs.(*RefTree); ok {
		return v.Ref
	}
	return "HEAD"
}

func rfcAsks(src RFCSource, today time.Time) ([]*Ask, []StaleDraft, error) {
	t := NewRefTree(src.Git, src.Ref)
	files, err := t.Files("")
	if err != nil {
		return nil, nil, err
	}
	touched := src.Git.PathDates(src.Ref, "*docs/rfcs/*.md", false)
	type rfc struct{ path, number string }
	var want []rfc
	var paths []string
	for _, p := range files {
		m := rfcPath.FindStringSubmatch(p)
		if m == nil || strings.Contains(p, "/archive/") {
			continue
		}
		want = append(want, rfc{p, m[1]})
		paths = append(paths, p)
	}
	if err := t.Prefetch(paths); err != nil {
		return nil, nil, err
	}
	var asks []*Ask
	var drafts []StaleDraft
	for _, r := range want {
		text, _, err := t.Read(r.path)
		if err != nil {
			return nil, nil, err
		}
		meta, _, yerr := SplitFrontmatter(text)
		if yerr != nil {
			meta = nil
		}
		status := ""
		if s := get(meta, "status"); truthy(s) {
			status = pyStr(s)
		}
		if status == "" {
			if sm := rfcStatus.FindStringSubmatch(text); sm != nil {
				status = sm[1]
			}
		}
		status = strings.ToLower(status)
		title := r.path
		for _, ln := range strings.Split(text, "\n") {
			if strings.HasPrefix(ln, "# ") {
				title = strings.TrimSpace(ln[2:])
				break
			}
		}
		last, haveLast := touched[r.path]
		switch status {
		case "considering":
			a := &Ask{
				ID:         "rfc:" + r.path,
				Repo:       src.Repo,
				Source:     "rfc",
				Kind:       "decide",
				Categories: []string{"rfc"},
				State:      "ready",
				Label:      rfcLabel(r.path, r.number),
				Text:       "Accept, revise or reject: " + title,
				Where:      r.path,
				Unblocks:   []string{},
			}
			if src.Repo != "" {
				a.ID = "rfc:" + src.Repo + ":" + r.path
			}
			if haveLast {
				s := isoDate(last)
				a.Raised = &s
			}
			asks = append(asks, a)
		case "draft":
			d := StaleDraft{Path: r.path, Repo: src.Repo, Title: title}
			if haveLast {
				n := int(today.Sub(last).Hours() / 24)
				d.IdleDays = &n
			}
			drafts = append(drafts, d)
		}
	}
	return asks, drafts, nil
}

// rfcLabel is `RFC 0092` for a root RFC, `search RFC 0003` for a project's.
func rfcLabel(p, number string) string {
	if i := strings.Index(p, "/docs/rfcs/"); i >= 0 {
		return p[:i] + " RFC " + number
	}
	return "RFC " + number
}

type askFile struct {
	path, slug string
	meta       map[string]any
	archived   bool
	answered   bool
}

// askFiles is (parsed ask files, errors): open asks in the project's asks/,
// answered ones in its archive/.
func (v *View) askFiles(p *Project) ([]askFile, []string) {
	var out []askFile
	var errors []string
	base := p.AsksDir()
	for _, sub := range []string{"", "archive"} {
		dir := base
		if sub != "" {
			dir += "/" + sub
		}
		var names []string
		if fs, ok := v.Files.(FSTree); ok {
			names = listDir(fs.Root, dir)
		} else {
			files, _ := v.Files.Files(dir)
			for _, f := range files {
				if rel := f[len(dir)+1:]; !strings.Contains(rel, "/") {
					names = append(names, rel)
				}
			}
		}
		for _, name := range names {
			if !strings.HasSuffix(name, ".md") || name == "README.md" {
				continue
			}
			rel := dir + "/" + name
			text, _, err := v.Files.Read(rel)
			if err != nil {
				continue
			}
			meta, _, yerr := SplitFrontmatter(text)
			if yerr != nil {
				errors = append(errors, fmt.Sprintf("%s: frontmatter is not valid YAML (%s)", rel, firstLine(yerr.Error())))
				continue
			}
			if len(meta) == 0 {
				errors = append(errors, rel+": no YAML frontmatter")
				continue
			}
			out = append(out, askFile{path: rel, slug: strings.TrimSuffix(name, ".md"), meta: meta,
				archived: sub == "archive", answered: truthy(get(meta, "answered"))})
		}
	}
	return out, errors
}

// CheckAsks validates ask files and human gate entries.
func (v *View) CheckAsks(only string) (errors, warnings []string, err error) {
	for _, p := range v.Projects {
		if only != "" && p.Name != only {
			continue
		}
		files, errs := v.askFiles(p)
		errors = append(errors, errs...)
		for _, a := range files {
			m, rel := a.meta, a.path
			if strings.TrimSpace(pyStr(orDefault(get(m, "ask"), ""))) == "" {
				errors = append(errors, rel+": `ask` is missing")
			}
			if k, ok := get(m, "kind").(string); !ok || !contains(askKinds, k) {
				errors = append(errors, fmt.Sprintf("%s: kind %s is not one of %s", rel, pyRepr(get(m, "kind")), strings.Join(askKinds, ", ")))
			}
			if _, ok := asDate(get(m, "raised")); !ok {
				errors = append(errors, rel+": `raised` must be a date (YYYY-MM-DD)")
			}
			if truthy(get(m, "defer_until")) {
				if _, ok := asDate(get(m, "defer_until")); !ok {
					errors = append(errors, rel+": `defer_until` must be a date")
				}
			}
			errors = append(errors, checkShape(m, rel)...)
			if a.answered {
				if _, ok := asDate(get(m, "answered")); !ok {
					errors = append(errors, rel+": `answered` must be a date")
				}
				if o, ok := get(m, "outcome").(string); !ok || !contains(askOutcomes, o) {
					errors = append(errors, fmt.Sprintf("%s: an answered ask needs outcome, one of %s", rel, strings.Join(askOutcomes, ", ")))
				}
				if _, hasOptions := m["options"]; !hasOptions && strings.TrimSpace(pyStr(orDefault(get(m, "answer"), ""))) == "" {
					errors = append(errors, rel+": an answered ask needs `answer`")
				}
				if !a.archived {
					warnings = append(warnings, fmt.Sprintf("%s: answered; git mv it to %s/archive/", rel, p.AsksDir()))
				}
			} else if a.archived {
				errors = append(errors, rel+": in archive/ but has no `answered` date")
			}
		}
	}
	found, err := v.AllDocs(only)
	if err != nil {
		return nil, nil, err
	}
	for _, g := range v.dispatchGates(IndexDocs(found)) {
		doc := g.doc
		if doc.Status != "active" && doc.Status != "" {
			continue
		}
		if g.fields != nil {
			where := doc.Path + ": human gate " + pyStrRepr(g.text)
			if strings.TrimSpace(g.text) == "" {
				errors = append(errors, doc.Path+": a human gate written as a mapping needs `ask`")
			}
			if k, ok := g.fields["kind"]; ok && !contains(askKinds, strOf(k)) {
				errors = append(errors, fmt.Sprintf("%s: kind %s is not one of %s", where, pyRepr(k), strings.Join(askKinds, ", ")))
			}
			errors = append(errors, checkShape(g.fields, where)...)
		}
		if g.task == "" {
			warnings = append(warnings, fmt.Sprintf("%s: human gate %s names no task (\"Task N: ...\")", doc.Path, pyStrRepr(g.text)))
		} else if _, ok := g.marks[g.task]; !ok {
			warnings = append(warnings, fmt.Sprintf("%s: human gate names Task %s, which has no `## Task` heading", doc.Path, g.task))
		}
	}
	plans, err := v.ActivePlans()
	if err != nil {
		return nil, nil, err
	}
	for _, p := range plans {
		if only != "" && p.Project != only {
			continue
		}
		for _, id := range p.Gates.keys {
			errors = append(errors, checkShape(p.Gates.byID[id], p.Path+": gate "+id)...)
		}
	}
	return errors, warnings, nil
}

func asDate(v any) (time.Time, bool) {
	if v == nil {
		return time.Time{}, false
	}
	return parseDate(pyStr(v))
}

// asStrings is scripts/asks' as_list: nothing, a list, or one value.
func asStrings(v any) []string {
	out := []string{}
	switch x := v.(type) {
	case nil:
	case []any:
		for _, e := range x {
			out = append(out, pyStr(e))
		}
	default:
		out = append(out, pyStr(x))
	}
	return out
}

// LocalRFCSource is the store's own repository, for the legacy layout.
func (v *View) LocalRFCSource() RFCSource {
	return RFCSource{Project: v.Projects[0].Name, Git: v.Store.Git, Ref: v.gitRef()}
}

// RepoCheckouts finds checkouts of the repositories projects own, for
// their RFCs: ORDVIK_REPOS ("owner/name=/path,..."), else a sibling of the
// planning repository named after the repository.
func (v *View) RepoCheckouts(only string) ([]RFCSource, []string) {
	explicit := map[string]string{}
	for _, e := range strings.Split(os.Getenv("ORDVIK_REPOS"), ",") {
		if k, val, ok := strings.Cut(strings.TrimSpace(e), "="); ok {
			explicit[k] = val
		}
	}
	var out []RFCSource
	var notes []string
	for _, p := range v.Projects {
		if only != "" && p.Name != only {
			continue
		}
		for _, repo := range p.Owns {
			dir, ok := explicit[repo]
			if !ok {
				name := repo[strings.LastIndex(repo, "/")+1:]
				dir = filepath.Join(filepath.Dir(v.Store.Root), name)
			}
			g := Git{Root: dir}
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				notes = append(notes, fmt.Sprintf("RFCs of %s not read: no checkout at %s (set ORDVIK_REPOS)", repo, dir))
				continue
			}
			out = append(out, RFCSource{Repo: repo, Project: p.Name, Git: g, Ref: g.DefaultRef()})
		}
	}
	return out, notes
}
