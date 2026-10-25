package planning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// marshalJSON encodes without HTML escaping, as Python's json does.
func marshalJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// Object is a JSON object that keeps its keys in the order they are set.
type Object struct {
	keys []string
	vals map[string]any
}

func NewObject() *Object { return &Object{vals: map[string]any{}} }

func (o *Object) Set(k string, v any) *Object {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
	return o
}

func (o *Object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("{")
	for i, k := range o.keys {
		if i > 0 {
			b.WriteString(",")
		}
		kb, _ := marshalJSON(k)
		vb, err := marshalJSON(o.vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteString(":")
		b.Write(vb)
	}
	b.WriteString("}")
	return b.Bytes(), nil
}

// MarshalJSON writes an ask with scripts/asks' keys for its source.
func (a *Ask) MarshalJSON() ([]byte, error) {
	o := NewObject().Set("id", a.ID)
	if a.Project != "" {
		o.Set("project", a.Project)
	}
	if a.Repo != "" {
		o.Set("repo", a.Repo)
	}
	o.Set("source", a.Source).Set("kind", a.Kind).Set("categories", a.Categories).Set("state", a.State)
	if a.hasDeferred {
		o.Set("deferred_until", a.DeferredUntil)
	}
	if a.Unmarked != nil {
		o.Set("unmarked", *a.Unmarked)
	}
	o.Set("label", a.Label).Set("text", a.Text).Set("where", a.Where)
	if a.Title != nil {
		o.Set("title", *a.Title)
	}
	if a.From != nil {
		o.Set("from", *a.From)
	}
	o.Set("unblocks", a.Unblocks).Set("raised", a.Raised).Set("age_days", a.AgeDays)
	return o.MarshalJSON()
}

// WriteJSON writes v indented by two spaces, then a newline.
func WriteJSON(w io.Writer, v any) error {
	b, err := marshalJSON(v)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "  "); err != nil {
		return err
	}
	out.WriteString("\n")
	_, err = w.Write(out.Bytes())
	return err
}

// StatusJSON is `plan status --json`.
func StatusJSON(pl *Plan, threads []*ThreadState, gates []PendingGate, ref, sha string, repo bool) *Object {
	o := NewObject().Set("plan", pl.Slug)
	if repo {
		o.Set("project", pl.Project)
	}
	if threads == nil {
		threads = []*ThreadState{}
	}
	return o.Set("status", pl.Status).Set("conductor", pl.Conductor).Set("ref", ref).
		Set("commit", sha).Set("threads", threads).Set("gates", gates)
}

func padRight(s string, n int) string {
	if l := len([]rune(s)); l < n {
		return s + strings.Repeat(" ", n-l)
	}
	return s
}

func padLeft(s string, n int) string {
	if l := len([]rune(s)); l < n {
		return strings.Repeat(" ", n-l) + s
	}
	return s
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// PrintStatus is `plan status` for people.
func PrintStatus(w io.Writer, pl *Plan, threads []*ThreadState, gates []PendingGate, ref, sha string, repo bool) {
	name := pl.Slug
	if repo {
		name = pl.Project + "/" + pl.Slug
	}
	fmt.Fprintf(w, "Plan %s (%s) as of %s %s\n", name, pl.Status, ref, short(sha))
	if truthy(pl.Title) {
		fmt.Fprintf(w, "  %s\n", pyStr(pl.Title))
	}
	if truthy(pl.Conductor) {
		fmt.Fprintf(w, "  conductor: %s\n", pyStr(pl.Conductor))
	}
	stageSet := map[int]bool{}
	for _, t := range threads {
		stageSet[t.Stage] = true
	}
	var stages []int
	for s := range stageSet {
		stages = append(stages, s)
	}
	sort.Ints(stages)
	for _, n := range stages {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Stage %d\n", n)
		for _, t := range threads {
			if t.Stage != n {
				continue
			}
			progress := ""
			if t.Tasks > 0 {
				progress = fmt.Sprintf("%d/%d tasks", t.TasksDone, t.Tasks)
			}
			line := fmt.Sprintf("  %s %s %s %s", padRight(t.ID, 5), padRight(t.State, 10), padRight(progress, 11), t.Title)
			fmt.Fprintln(w, strings.TrimRight(line, " \t\n"))
			if len(t.WaitingOn) > 0 && (t.State == "blocked" || t.State == "planned") {
				fmt.Fprintf(w, "        waits on: %s\n", strings.Join(t.WaitingOn, ", "))
			}
			if t.State == "ready" || t.State == "started" || t.State == "blocked" || t.State == "write-doc" {
				for _, m := range t.Milestones {
					if m.Name != "done" {
						fmt.Fprintf(w, "        %s unblocks: %s\n", m.Name, strings.Join(m.IDs, ", "))
					}
				}
			}
		}
	}
	section := func(heading string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s:\n", heading)
		for _, i := range items {
			fmt.Fprintf(w, "  %s\n", i)
		}
	}
	var launch, write, human, ask []string
	for _, t := range threads {
		doc := ""
		if t.Doc != nil {
			doc = *t.Doc
		}
		switch t.State {
		case "ready":
			launch = append(launch, t.ID+"  "+doc)
		case "write-doc":
			write = append(write, t.ID+"  "+t.Title)
		case "human":
			human = append(human, t.ID+"  "+doc)
		}
	}
	for _, g := range gates {
		ask = append(ask, fmt.Sprintf("%s: %s (unblocks %s)", g.ID, pyStr(g.Ask), strings.Join(g.Unblocks, ", ")))
	}
	section("Launch now", launch)
	section("Write the dispatch doc", write)
	section("Human-executed, ready", human)
	section("Gates to ask the operator", ask)
	open := 0
	for _, t := range threads {
		if t.State != "done" && t.State != "superseded" && t.State != "dropped" {
			open++
		}
	}
	if open == 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Every thread is done: close the plan.")
	}
}

// AsksJSON is `asks status --json`.
func AsksJSON(res *AsksResult, ref, sha string, today time.Time) *Object {
	rec := NewObject()
	for _, m := range res.Recurring {
		rec.Set(m.Name, m.IDs)
	}
	o := NewObject().Set("ref", ref).Set("commit", sha).Set("today", isoDate(today)).
		Set("asks", res.Asks).Set("recurring", rec).Set("rfc_drafts", res.Drafts)
	if len(res.Notes) > 0 {
		o.Set("notes", res.Notes)
	}
	return o
}

var askSections = [][2]string{{"decide", "Decide"}, {"review", "Review"}, {"approve", "Approve"}, {"do", "Do"}}

// PrintAsks is `asks status` for people.
func PrintAsks(w io.Writer, res *AsksResult, ref, sha string, today time.Time, shallow bool) {
	fmt.Fprintf(w, "Asks for the operator as of %s %s (%s)\n", ref, short(sha), isoDate(today))
	var open []*Ask
	width := 0
	for _, a := range res.Asks {
		if l := len([]rune(a.Label)); l > width {
			width = l
		}
		if a.State == "ready" {
			open = append(open, a)
		}
	}
	if len(res.Asks) == 0 {
		width = 10
	}
	if width > 32 {
		width = 32
	}
	for _, s := range askSections {
		var items []*Ask
		for _, a := range open {
			if a.Kind == s[0] {
				items = append(items, a)
			}
		}
		if len(items) == 0 {
			continue
		}
		sort.SliceStable(items, func(i, j int) bool { return askLess(items[i], items[j]) })
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s (%d)\n", s[1], len(items))
		for _, a := range items {
			age := ""
			if a.AgeDays != nil {
				age = fmt.Sprintf("%dd", *a.AgeDays)
			}
			flag := " "
			if a.unmarkedCount() > 0 {
				flag = "*"
			}
			fmt.Fprintf(w, " %s%s %s  %s\n", flag, padRight(a.Label, width), padLeft(age, 5), pyStr(a.Text))
			var extra []string
			if len(a.Unblocks) > 0 {
				extra = append(extra, "unblocks "+strings.Join(a.Unblocks, ", "))
			}
			if a.From != nil && truthy(*a.From) {
				extra = append(extra, "from "+pyStr(*a.From))
			}
			extra = append(extra, a.Where)
			fmt.Fprintf(w, "  %s %s  %s\n", strings.Repeat(" ", width), strings.Repeat(" ", 5), strings.Join(extra, " · "))
		}
	}
	for _, a := range open {
		if a.unmarkedCount() > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "* Earlier tasks of that dispatch are not marked — DONE: the gate is not")
			fmt.Fprintln(w, "  reachable yet, or the doc is stale (mark the tasks, or close the dispatch).")
			break
		}
	}
	var deferred []*Ask
	for _, a := range res.Asks {
		if a.State == "deferred" {
			deferred = append(deferred, a)
		}
	}
	if len(deferred) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Deferred (%d)\n", len(deferred))
		for _, a := range deferred {
			until := ""
			if a.DeferredUntil != nil {
				until = *a.DeferredUntil
			}
			fmt.Fprintf(w, "  %s until %s  %s\n", padRight(a.Label, width), until, pyStr(a.Text))
		}
	}
	var cands []Milestone
	for _, m := range res.Recurring {
		if len(m.IDs) >= Recurring {
			cands = append(cands, m)
		}
	}
	if len(cands) > 0 {
		sort.SliceStable(cands, func(i, j int) bool {
			if len(cands[i].IDs) != len(cands[j].IDs) {
				return len(cands[i].IDs) > len(cands[j].IDs)
			}
			return cands[i].Name < cands[j].Name
		})
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Candidates to automate (%d+ open `do` asks of one kind)\n", Recurring)
		for _, c := range cands {
			var short []string
			for _, id := range c.IDs {
				_, rest, _ := strings.Cut(id, ":")
				short = append(short, rest)
			}
			fmt.Fprintf(w, "  %s %s  %s\n", padRight(c.Name, 20), padLeft(fmt.Sprint(len(c.IDs)), 2), strings.Join(short, ", "))
		}
	}
	fmt.Fprintln(w)
	line := fmt.Sprintf("RFC drafts: %d, %d untouched for %d+ days", res.Drafts.Count, res.Drafts.StaleCount, StaleDraftDays)
	if len(res.Drafts.Stale) == 0 && res.Drafts.StaleCount > 0 {
		line += " (--all lists them)"
	}
	fmt.Fprintln(w, line)
	stale := append([]StaleDraft{}, res.Drafts.Stale...)
	sort.SliceStable(stale, func(i, j int) bool { return *stale[i].IdleDays > *stale[j].IdleDays })
	for _, d := range stale {
		p := d.Path
		if d.Repo != "" {
			p = d.Repo + ":" + p
		}
		fmt.Fprintf(w, "  %sd  %s\n", padLeft(fmt.Sprint(*d.IdleDays), 4), p)
	}
	for _, n := range res.Notes {
		fmt.Fprintln(w, n)
	}
	if shallow {
		fmt.Fprintln(w, "Ages are unreliable: this clone is shallow (git fetch --unshallow origin).")
	}
	fmt.Fprintln(w, "Not in git: open PRs and blocked project threads (the sweep adds them).")
}

func askLess(a, b *Ask) bool {
	if len(a.Unblocks) != len(b.Unblocks) {
		return len(a.Unblocks) > len(b.Unblocks)
	}
	au, bu := a.unmarkedCount() > 0, b.unmarkedCount() > 0
	if au != bu {
		return !au
	}
	aa, ba := 0, 0
	if a.AgeDays != nil {
		aa = *a.AgeDays
	}
	if b.AgeDays != nil {
		ba = *b.AgeDays
	}
	if aa != ba {
		return aa > ba
	}
	return a.ID < b.ID
}
