package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/xythian/ordvik/planning"
)

// sleep waits between fetches in ask wait; tests replace it.
var sleep = time.Sleep

func ask(store *planning.Store, o opts, stdout, stderr io.Writer) int {
	if len(o.args) == 0 {
		fmt.Fprintln(stderr, "Error: ask takes new, show, wait, answer or defer.")
		return 2
	}
	sub, args := o.args[0], o.args[1:]
	switch sub {
	case "new":
		if len(args) > 0 {
			fmt.Fprintf(stderr, "Error: unexpected argument %q.\n", args[0])
			return 2
		}
		return askNew(store, o, stdout, stderr)
	case "show", "wait":
		if len(args) != 1 {
			fmt.Fprintf(stderr, "Error: ask %s takes one ask id.\n", sub)
			return 2
		}
		return askShow(store, o, args[0], sub == "wait", stdout, stderr)
	case "answer":
		if len(args) < 1 || len(args) > 2 {
			fmt.Fprintln(stderr, "Error: ask answer takes an ask id and an option's index (or other).")
			return 2
		}
		return askAnswer(store, o, args, stdout, stderr)
	case "defer":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "Error: ask defer takes an ask id and a date (YYYY-MM-DD).")
			return 2
		}
		return askDefer(store, o, args, stdout, stderr)
	}
	fmt.Fprintf(stderr, "Error: ask takes new, show, wait, answer or defer, not %q.\n", sub)
	return 2
}

func askNew(store *planning.Store, o opts, stdout, stderr io.Writer) int {
	if o.project == "" && o.gate == "" {
		fmt.Fprintln(stderr, "Error: ask new takes --project (or --gate).")
		return 2
	}
	if o.recommended == "" {
		fmt.Fprintln(stderr, "Error: ask new takes --recommended <n>: say which option you would choose.")
		return 2
	}
	rec, err := strconv.Atoi(o.recommended)
	if err != nil {
		fmt.Fprintln(stderr, "Error: --recommended takes an option's index, from 0.")
		return 2
	}
	t, err := today(o)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	n := planning.NewAsk{
		Project: o.project, Question: strings.TrimSpace(o.question), Kind: o.kind, Categories: o.categories,
		Recommended: rec, Context: o.context, Gate: o.gate, Raised: t,
		From: planning.AskFrom{Thread: o.fromThread, PR: o.fromPR, Dispatch: o.fromDisp},
	}
	for _, opt := range o.options {
		label, then, _ := strings.Cut(opt, "=")
		n.Options = append(n.Options, planning.AskOption{Label: strings.TrimSpace(label), Then: strings.TrimSpace(then)})
	}
	if err := planning.CheckNewAsk(n); err != nil {
		fmt.Fprintf(stderr, "Error: %v.\n", err)
		return 2
	}
	id, err := store.RaiseAsk(n)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Asked %s and pushed.\n", id)
	if view, err := store.NewView("", false); err == nil {
		if u := view.AskURL(id); u != "" {
			fmt.Fprintln(stdout, u)
		}
	}
	fmt.Fprintf(stdout, "Read the answer with: ordvik planning ask show %s (or ask wait)\n", id)
	return 0
}

func askShow(store *planning.Store, o opts, id string, wait bool, stdout, stderr io.Writer) int {
	t, err := today(o)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	repo := store.Layout == planning.LayoutRepo
	deadline := time.Now().Add(o.timeout)
	for {
		if o.fetch || wait || (repo && !o.offline && o.ref == "" && !o.worktree) {
			if err := store.Git.Fetch(); err != nil {
				fmt.Fprintf(stderr, "warning: %v; reading what this clone has\n", err)
			}
		}
		view, err := store.NewView(o.ref, o.worktree)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		a, err := view.FindAsk(id, t)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		if !wait || a.Answered != nil || !time.Now().Add(o.interval).Before(deadline) {
			if err := printAsk(stdout, a, o.json); err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return 1
			}
			if wait && a.Answered == nil {
				return 3
			}
			return 0
		}
		sleep(o.interval)
	}
}

func printAsk(w io.Writer, a *planning.Ask, asJSON bool) error {
	if asJSON {
		return planning.WriteJSON(w, a)
	}
	state := "open"
	if a.Answered != nil {
		state = "answered " + *a.Answered
	}
	fmt.Fprintf(w, "%s  %s, %s\n%s\n", a.ID, a.Kind, state, fmt.Sprint(a.Text))
	if len(a.Options) > 0 {
		fmt.Fprintln(w)
		for i, opt := range a.Options {
			rec := ""
			if a.Recommended != nil && *a.Recommended == i {
				rec = " (recommended)"
			}
			fmt.Fprintf(w, "  %d. %s%s: %s\n", i, opt.Label, rec, opt.Then)
		}
	}
	if a.Context != "" {
		fmt.Fprintf(w, "\n%s\n", a.Context)
	}
	fmt.Fprintln(w)
	if a.From != nil && *a.From != nil {
		fmt.Fprintf(w, "from: %s\n", fromText(*a.From))
	}
	fmt.Fprintf(w, "where: %s\n", a.Where)
	if a.Answered != nil {
		answer := fmt.Sprint(a.Answer)
		if n, ok := a.Answer.(int); ok && n >= 0 && n < len(a.Options) {
			answer = fmt.Sprintf("%d. %s", n, a.Options[n].Label)
		}
		if a.AnswerText != "" {
			answer += ": " + a.AnswerText
		}
		by := ""
		if a.AnsweredBy != "" {
			by = " by " + a.AnsweredBy
		}
		fmt.Fprintf(w, "answer (%s%s): %s\n", *a.Answered, by, answer)
	}
	return nil
}

func fromText(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Sprint(v)
	}
	var parts []string
	for _, k := range []string{"thread", "pr", "dispatch"} {
		if s, ok := m[k]; ok {
			parts = append(parts, fmt.Sprintf("%s %v", k, s))
		}
	}
	return strings.Join(parts, " · ")
}

func askAnswer(store *planning.Store, o opts, args []string, stdout, stderr io.Writer) int {
	t, err := today(o)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	a := planning.AskAnswer{ID: args[0], Text: o.text, By: o.by, Outcome: o.outcome, Date: t}
	if len(args) > 1 {
		a.Answer = args[1]
	}
	if a.By == "" {
		a.By = store.UserName()
	}
	id, err := store.AnswerAsk(a)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Answered %s and pushed.\n", id)
	return 0
}

func askDefer(store *planning.Store, o opts, args []string, stdout, stderr io.Writer) int {
	t, err := today(o)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	until, err := time.Parse("2006-01-02", args[1])
	if err != nil {
		fmt.Fprintf(stderr, "Error: %q is not a date (YYYY-MM-DD).\n", args[1])
		return 2
	}
	id, err := store.DeferAsk(args[0], until, t)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Deferred %s to %s and pushed.\n", id, args[1])
	return 0
}
