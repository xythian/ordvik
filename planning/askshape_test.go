package planning

import (
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func decodeMap(t *testing.T, text string) map[string]any {
	t.Helper()
	v, err := decodeYAML(text)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := v.(map[string]any)
	return m
}

const shapeOK = `ask: "Ship it?"
options:
  - {label: "Yes", then: "it ships"}
  - {label: "No", then: "it waits"}
recommended: 0
context: "Why now."
from: {thread: "a thread", pr: "acme/app#3", dispatch: "demo/20261028-x"}
`

func TestCheckShape(t *testing.T) {
	long := strings.Repeat("word ", MaxContextWords+1)
	for _, c := range []struct {
		name, text, want string
	}{
		{"valid", shapeOK, ""},
		{"old ask file", "ask: Export files\nkind: do\nfrom: \"a thread\"\nraised: 2026-10-20\n", ""},
		{"old answered ask file", "ask: x\nanswered: 2026-10-21\nanswer: done in PR 4\noutcome: done\n", ""},
		{"one option", "options:\n  - {label: a, then: b}\n", "2 to 4 options"},
		{"five options", "options: [{label: a, then: b}, {label: a, then: b}, {label: a, then: b}, {label: a, then: b}, {label: a, then: b}]\n", "2 to 4 options"},
		{"option without then", "options: [{label: a, then: b}, {label: c}]\n", "option 1 needs `then`"},
		{"option not a mapping", "options: [{label: a, then: b}, c]\n", "option 1 is not"},
		{"recommended out of range", strings.Replace(shapeOK, "recommended: 0", "recommended: 2", 1), "`recommended` must be an option's index, 0 to 1"},
		{"recommended not a number", strings.Replace(shapeOK, "recommended: 0", "recommended: yes", 1), "`recommended` must be"},
		{"recommended without options", "recommended: 0\n", "`recommended` without `options`"},
		{"long context", "context: \"" + long + "\"\n", "keep it to 120"},
		{"from with another key", "from: {thread: a, branch: b}\n", "not branch"},
		{"from with a bad PR", "from: {pr: acme/app}\n", "`from.pr`"},
		{"answered with an index", shapeOK + "answered: 2026-10-28\nanswer: 1\n", ""},
		{"answered other with text", shapeOK + "answered: 2026-10-28\nanswer: other\nanswer_text: later\n", ""},
		{"answered without an answer", shapeOK + "answered: 2026-10-28\n", "names its answer"},
		{"answered out of range", shapeOK + "answered: 2026-10-28\nanswer: 3\n", "`answer` 3 is not"},
		{"answered other without text", shapeOK + "answered: 2026-10-28\nanswer: other\n", "needs `answer_text`"},
		{"answered with a word", shapeOK + "answered: 2026-10-28\nanswer: yes please\n", "an option's index or other"},
		{"answered not a date", shapeOK + "answered: soon\nanswer: 0\n", "`answered` must be a date"},
		{"answer without answered", shapeOK + "answer: 0\n", "without an `answered` date"},
	} {
		errs := checkShape(decodeMap(t, c.text), "f.md")
		got := strings.Join(errs, "\n")
		if c.want == "" && len(errs) > 0 {
			t.Errorf("%s: unexpected errors:\n%s", c.name, got)
		}
		if c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: want an error with %q, got:\n%s", c.name, c.want, got)
		}
	}
}

func TestParseShape(t *testing.T) {
	s := parseShape(decodeMap(t, shapeOK+"answered: 2026-10-28\nanswer: 1\nanswered_by: Op\n"))
	if len(s.Options) != 2 || s.Options[1] != (AskOption{"No", "it waits"}) || s.Recommended == nil || *s.Recommended != 0 {
		t.Fatalf("shape = %+v", s)
	}
	if s.Context != "Why now." || s.Answered != "2026-10-28" || s.Answer != 1 || s.AnsweredBy != "Op" {
		t.Fatalf("shape = %+v", s)
	}
	if s := parseShape(nil); s.Options != nil || s.Answered != "" {
		t.Fatalf("empty shape = %+v", s)
	}
}

func TestGateFields(t *testing.T) {
	if text, m := gateFields("Task 2: Deploy"); text != "Task 2: Deploy" || m != nil {
		t.Fatalf("string gate = %q %v", text, m)
	}
	text, m := gateFields(decodeMap(t, "ask: \"Task 3: Ship it?\"\nrecommended: 0\n"))
	if text != "Task 3: Ship it?" || m == nil {
		t.Fatalf("mapping gate = %q %v", text, m)
	}
}

func TestParseAskRef(t *testing.T) {
	for in, want := range map[string]string{
		"ask:demo/ship-it":         "ask:demo/ship-it",
		"demo/ship-it":             "ask:demo/ship-it",
		"ship-it":                  "ask:ship-it",
		"dispatch:demo/20261028:2": "dispatch:demo/20261028:2",
		"demo/20261028:g1":         "dispatch:demo/20261028:G1",
		"plan:demo/rollout:go":     "plan:demo/rollout:go",
	} {
		r, err := parseAskRef(in)
		if err != nil || r.id() != want {
			t.Errorf("parseAskRef(%q) = %q, %v; want %q", in, r.id(), err, want)
		}
	}
	for _, bad := range []string{"plan:demo:go", "dispatch:demo/x", "ask:", "a/b/c"} {
		if _, err := parseAskRef(bad); err == nil {
			t.Errorf("parseAskRef(%q) took it", bad)
		}
	}
}

func TestAskSlug(t *testing.T) {
	for q, want := range map[string]string{
		"Ship the parser behind a flag?":                                     "ship-the-parser-behind-a-flag",
		"Should ops-01 hold sudo for planning reads, or approve each scope?": "should-ops-01-hold-sudo-for-planning-reads-or",
		"???": "ask",
	} {
		if got := askSlug(q); got != want {
			t.Errorf("askSlug(%q) = %q, want %q", q, got, want)
		}
	}
}

func TestEditListItemKeepsOtherLines(t *testing.T) {
	text := "---\nid: x\n# a comment\nhuman_gates:\n  - \"Task 1: Deploy\"\n  - \"Task 2: Review\"\nprs:\n  - {pr: \"a/b#1\"}\n---\n\nbody\n"
	out, err := editListItem(text, "human_gates", dispatchGateMatch("1"), func(n *yaml.Node) (*yaml.Node, error) {
		m := gateMapping(n)
		setKey(m, "recommended", plainNode("!!int", "0"))
		return m, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nid: x\n# a comment\nhuman_gates:\n  - ask: \"Task 1: Deploy\"\n    recommended: 0\n  - \"Task 2: Review\"\nprs:\n  - {pr: \"a/b#1\"}\n---\n\nbody\n"
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	// No gate list yet: one is added before the closing ---.
	out, err = editListItem("---\nid: x\n---\n", "human_gates", dispatchGateMatch("2"), func(n *yaml.Node) (*yaml.Node, error) {
		return gateMapping(strNode("Task 2: Ship?")), nil
	})
	if err != nil || out != "---\nid: x\nhuman_gates:\n  - ask: \"Task 2: Ship?\"\n---\n" {
		t.Fatalf("added: %v\n%s", err, out)
	}
	if _, err := editListItem("---\nhuman_gates: [\"Task 1: a\"]\n---\n", "human_gates", dispatchGateMatch("1"),
		func(n *yaml.Node) (*yaml.Node, error) { return n, nil }); err == nil || !strings.Contains(err.Error(), "inline") {
		t.Fatalf("inline list: %v", err)
	}
}
