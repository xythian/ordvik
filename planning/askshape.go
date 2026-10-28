package planning

import (
	"fmt"
	"strings"
)

// An ask can carry the shape the operator answers with one tap: a
// question, two to four options (each with what happens if it is
// chosen), the option recommended, and a few sentences of context. Ask
// files, dispatch gates and plan gates take the same optional fields:
//
//	options       [{label, then}], 2 to 4
//	recommended   the recommended option's index, from 0
//	context       why it is asked now and what each answer changes
//	from          {thread, pr, dispatch}: where the ask came from
//
// and, once answered:
//
//	answered      the date
//	answer        the chosen option's index, or `other`
//	answer_text   the operator's words (required with `other`)
//	answered_by   who answered
//
// An ask file with no options keeps its one-line `answer` text.

const (
	MinOptions      = 2
	MaxOptions      = 4
	MaxContextWords = 120
)

var fromKeys = []string{"thread", "pr", "dispatch"}

// AskOption is one choice of an ask.
type AskOption struct {
	Label string `json:"label"`
	Then  string `json:"then"`
}

// AskShape is an ask's options, recommendation, context and answer.
type AskShape struct {
	Options     []AskOption
	Recommended *int
	Context     string
	Answered    string
	Answer      any // the option's index (int), "other", or an old ask file's text
	AnswerText  string
	AnsweredBy  string
}

// parseShape reads the shape's fields from an ask file's frontmatter or a
// gate's mapping; fields that do not parse are left out (check reports
// them).
func parseShape(m map[string]any) AskShape {
	var s AskShape
	for _, o := range asList(get(m, "options")) {
		om, _ := o.(map[string]any)
		s.Options = append(s.Options, AskOption{Label: strOf(get(om, "label")), Then: strOf(get(om, "then"))})
	}
	if n, ok := get(m, "recommended").(int64); ok {
		i := int(n)
		s.Recommended = &i
	}
	s.Context = strings.TrimSpace(strOf(get(m, "context")))
	if truthy(get(m, "answered")) {
		s.Answered = pyStr(get(m, "answered"))
	}
	switch a := get(m, "answer").(type) {
	case nil:
	case int64:
		s.Answer = int(a)
	default:
		s.Answer = pyStr(a)
	}
	s.AnswerText = strings.TrimSpace(strOf(get(m, "answer_text")))
	s.AnsweredBy = strOf(get(m, "answered_by"))
	return s
}

// strOf is a string field's value; anything else, or nothing, is "".
func strOf(v any) string {
	if v == nil {
		return ""
	}
	return pyStr(v)
}

// checkShape is the errors in the shape's fields of one ask file or gate,
// each prefixed with where.
func checkShape(m map[string]any, where string) []string {
	var errs []string
	add := func(format string, a ...any) { errs = append(errs, where+": "+fmt.Sprintf(format, a...)) }
	_, hasOptions := m["options"]
	options := asList(get(m, "options"))
	if hasOptions {
		if len(options) < MinOptions || len(options) > MaxOptions {
			add("`options` must list %d to %d options", MinOptions, MaxOptions)
		}
		for i, o := range options {
			om, ok := o.(map[string]any)
			if !ok {
				add("option %d is not a {label, then} mapping", i)
				continue
			}
			for _, k := range []string{"label", "then"} {
				if _, ok := get(om, k).(string); !ok || strings.TrimSpace(strOf(get(om, k))) == "" {
					add("option %d needs `%s`", i, k)
				}
			}
		}
	}
	if r, ok := m["recommended"]; ok {
		n, isInt := r.(int64)
		switch {
		case !hasOptions:
			add("`recommended` without `options`")
		case !isInt || n < 0 || int(n) >= len(options):
			add("`recommended` must be an option's index, 0 to %d", len(options)-1)
		}
	}
	if c, ok := m["context"]; ok {
		if _, isStr := c.(string); !isStr {
			add("`context` must be text")
		} else if n := len(strings.Fields(c.(string))); n > MaxContextWords {
			add("`context` is %d words; keep it to %d", n, MaxContextWords)
		}
	}
	switch f := get(m, "from").(type) {
	case nil, string:
	case map[string]any:
		for k, v := range f {
			if !contains(fromKeys, k) {
				add("`from` takes %s, not %s", strings.Join(fromKeys, ", "), k)
				continue
			}
			if k == "pr" {
				if _, _, ok := ParsePRRef(strOf(v)); !ok {
					add("`from.pr` %s is not owner/name#number", pyRepr(v))
				}
			}
		}
	default:
		add("`from` must be text or {thread, pr, dispatch}")
	}
	if truthy(get(m, "answered")) {
		if _, ok := asDate(get(m, "answered")); !ok {
			add("`answered` must be a date")
		}
		if hasOptions {
			switch a := get(m, "answer").(type) {
			case int64:
				if a < 0 || int(a) >= len(options) {
					add("`answer` %d is not an option's index", a)
				}
			case string:
				if a != "other" {
					add("`answer` must be an option's index or other")
				} else if strings.TrimSpace(strOf(get(m, "answer_text"))) == "" {
					add("an `other` answer needs `answer_text`")
				}
			default:
				add("an answered ask names its answer: an option's index or other")
			}
		}
	} else if _, ok := m["answer"]; ok && hasOptions {
		add("`answer` without an `answered` date")
	}
	return errs
}

// setShape copies the shape onto an ask for its JSON.
func (a *Ask) setShape(s AskShape) {
	if len(s.Options) > 0 {
		a.Options = s.Options
		a.Recommended = s.Recommended
	}
	a.Context = s.Context
	if s.Answered != "" {
		a.Answered = &s.Answered
		a.Answer = s.Answer
		a.AnswerText = s.AnswerText
		a.AnsweredBy = s.AnsweredBy
	}
}

// gateFields is a gate entry's text and fields: a dispatch's human_gates
// entry is a string, or a mapping whose `ask` is that string and which
// carries the shape.
func gateFields(e any) (string, map[string]any) {
	if m, ok := e.(map[string]any); ok {
		return strOf(get(m, "ask")), m
	}
	return pyStr(e), nil
}
