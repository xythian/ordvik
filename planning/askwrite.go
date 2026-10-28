package planning

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Raising and answering asks: `ordvik planning ask new | answer` write to
// the planning repository through Publish, so an agent anywhere that can
// push to it can ask the operator and read the answer back.

// AskFrom says where an ask came from.
type AskFrom struct {
	Thread   string
	PR       string // owner/name#n
	Dispatch string // <project>/<id>
}

// NewAsk is what `ask new` raises.
type NewAsk struct {
	Project     string
	Question    string
	Kind        string // default decide
	Categories  []string
	Options     []AskOption
	Recommended int
	Context     string
	From        AskFrom
	// Gate, when set, raises the ask on an existing gate instead of in a
	// new file: <project>/<dispatch>:<task>, or plan:<project>/<plan>:<gate>.
	Gate   string
	Raised time.Time
}

// AskAnswer is what `ask answer` records.
type AskAnswer struct {
	ID      string
	Answer  string // an option's index, `other`, or "" for an ask without options
	Text    string
	By      string
	Outcome string // ask files: default decided (done when the ask has no options)
	Date    time.Time
}

// askRef is a parsed ask id.
type askRef struct {
	source  string // ask, dispatch or plan
	project string // "" for a bare ask slug
	name    string // the ask's slug, the dispatch id or the plan slug
	sub     string // the task or the gate id
}

func (r askRef) id() string {
	switch r.source {
	case "ask":
		if r.project == "" {
			return "ask:" + r.name
		}
		return "ask:" + r.project + "/" + r.name
	case "dispatch":
		return "dispatch:" + r.project + "/" + r.name + ":" + r.sub
	}
	return "plan:" + r.project + "/" + r.name + ":" + r.sub
}

// parseAskRef reads an ask id as `asks --json` prints it, or shorter:
// <project>/<slug> or <slug> for an ask file, <project>/<dispatch>:<task>
// for a dispatch gate.
func parseAskRef(id string) (askRef, error) {
	source, rest, ok := strings.Cut(id, ":")
	if !ok || (source != "ask" && source != "dispatch" && source != "plan") {
		source, rest = "ask", id
		if strings.Contains(id, ":") {
			source = "dispatch"
		}
	}
	bad := fmt.Errorf("%q is not an ask id (ask:<project>/<slug>, dispatch:<project>/<id>:<task> or plan:<project>/<plan>:<gate>)", id)
	r := askRef{source: source}
	if source == "ask" {
		r.project, r.name, ok = strings.Cut(rest, "/")
		if !ok {
			r.project, r.name = "", rest
		}
		if r.name == "" || strings.ContainsAny(r.name, "/:") {
			return r, bad
		}
		return r, nil
	}
	key, sub, ok := strings.Cut(rest, ":")
	if !ok || sub == "" {
		return r, bad
	}
	r.project, r.name, ok = strings.Cut(key, "/")
	if !ok || r.project == "" || r.name == "" {
		return r, bad
	}
	r.sub = sub
	if source == "dispatch" {
		r.sub = strings.ToUpper(strings.TrimPrefix(strings.TrimPrefix(sub, "Task "), "task "))
	}
	return r, nil
}

// FindAsk is the ask id names, open or answered.
func (v *View) FindAsk(id string, today time.Time) (*Ask, error) {
	r, err := parseAskRef(id)
	if err != nil {
		return nil, err
	}
	res, err := v.gather(today, false, nil, r.project, true)
	if err != nil {
		return nil, err
	}
	want := r.id()
	for _, a := range res.Asks {
		if a.ID == want || (r.source == "ask" && r.project == "" && strings.HasSuffix(a.ID, "/"+r.name)) {
			return a, nil
		}
	}
	return nil, fmt.Errorf("no ask %s (a gate whose task is marked done is closed)", want)
}

// AskURL is the link the planning repository's planning.yml gives an ask
// (`ask_url`, with {id} for its id), or "".
func (v *View) AskURL(id string) string {
	text, ok, err := v.Files.Read(repoMarker)
	if err != nil || !ok {
		return ""
	}
	val, err := decodeYAML(text)
	if err != nil {
		return ""
	}
	m, _ := val.(map[string]any)
	tpl := strOf(get(m, "ask_url"))
	if tpl == "" {
		return ""
	}
	if !strings.Contains(tpl, "{id}") {
		return strings.TrimSuffix(tpl, "/") + "/" + id
	}
	return strings.ReplaceAll(tpl, "{id}", id)
}

// CheckNewAsk is what is wrong with n before anything is written.
func CheckNewAsk(n NewAsk) error {
	switch {
	case strings.TrimSpace(n.Question) == "":
		return fmt.Errorf("an ask needs a question")
	case len(n.Options) < MinOptions || len(n.Options) > MaxOptions:
		return fmt.Errorf("an ask needs %d to %d options", MinOptions, MaxOptions)
	case n.Recommended < 0 || n.Recommended >= len(n.Options):
		return fmt.Errorf("--recommended is an option's index, 0 to %d", len(n.Options)-1)
	case n.Kind != "" && !contains(askKinds, n.Kind):
		return fmt.Errorf("--kind is one of %s", strings.Join(askKinds, ", "))
	case len(strings.Fields(n.Context)) > MaxContextWords:
		return fmt.Errorf("the context is %d words; keep it to %d", len(strings.Fields(n.Context)), MaxContextWords)
	}
	for i, o := range n.Options {
		if strings.TrimSpace(o.Label) == "" || strings.TrimSpace(o.Then) == "" {
			return fmt.Errorf("option %d needs a label and what happens if it is chosen (\"label=then\")", i)
		}
	}
	if n.From.PR != "" {
		if _, _, ok := ParsePRRef(n.From.PR); !ok {
			return fmt.Errorf("--from-pr %q is not owner/name#number", n.From.PR)
		}
	}
	return nil
}

// RaiseAsk writes n to the planning repository and pushes it. It returns
// the ask's id.
func (s *Store) RaiseAsk(n NewAsk) (string, error) {
	if err := CheckNewAsk(n); err != nil {
		return "", err
	}
	if n.Gate != "" {
		return s.raiseOnGate(n)
	}
	if n.Kind == "" {
		n.Kind = "decide"
	}
	var id string
	_, err := s.Publish(n.Project+": ask "+askSlug(n.Question), func(v *View) ([]string, error) {
		p := v.Project(n.Project)
		if p == nil {
			return nil, fmt.Errorf("no project %s", n.Project)
		}
		slug := uniqueSlug(v.Store.Root, p.AsksDir(), askSlug(n.Question))
		text, err := askFileText(n)
		if err != nil {
			return nil, err
		}
		rel := p.AsksDir() + "/" + slug + ".md"
		if err := writeFile(v.Store.Root, rel, text); err != nil {
			return nil, err
		}
		id = askRef{source: "ask", project: p.Name, name: slug}.id()
		return []string{rel}, nil
	})
	return id, err
}

// raiseOnGate gives an existing gate the ask's question and shape, or adds
// the gate to a dispatch doc that does not have it yet.
func (s *Store) raiseOnGate(n NewAsk) (string, error) {
	r, err := gateRef(n.Gate)
	if err != nil {
		return "", err
	}
	_, err = s.Publish(fmt.Sprintf("%s/%s: ask on gate %s", r.project, r.name, r.sub), func(v *View) ([]string, error) {
		t, err := v.gateTarget(r)
		if err != nil {
			return nil, err
		}
		text, err := editListItem(t.text, t.key, t.match, func(node *yaml.Node) (*yaml.Node, error) {
			if node == nil {
				if r.source == "plan" {
					return nil, fmt.Errorf("%s has no gate %s", t.path, r.sub)
				}
				node = strNode("Task " + r.sub + ": " + n.Question)
			}
			var m *yaml.Node
			if r.source == "plan" {
				m = node
				setKey(m, "ask", strNode(n.Question))
			} else {
				m = gateMapping(node)
				setKey(m, "ask", strNode("Task "+r.sub+": "+n.Question))
			}
			shapeNodes(m, n)
			return m, nil
		})
		if err != nil {
			return nil, fmt.Errorf("%s: %v", t.path, err)
		}
		if err := writeFile(v.Store.Root, t.path, text); err != nil {
			return nil, err
		}
		return []string{t.path}, nil
	})
	return r.id(), err
}

// AnswerAsk records an answer and pushes it: an ask file gains its answer
// and moves to the archive, a gate gains its answer in place.
func (s *Store) AnswerAsk(a AskAnswer) (string, error) {
	r, err := parseAskRef(a.ID)
	if err != nil {
		return "", err
	}
	var id string
	_, err = s.Publish("answer "+r.id(), func(v *View) ([]string, error) {
		if r.source != "ask" {
			t, err := v.gateTarget(r)
			if err != nil {
				return nil, err
			}
			id = r.id()
			text, err := editListItem(t.text, t.key, t.match, func(node *yaml.Node) (*yaml.Node, error) {
				if node == nil {
					return nil, fmt.Errorf("no gate %s", r.id())
				}
				m := node
				if r.source == "dispatch" {
					m = gateMapping(node)
				}
				val, _ := nodeValue(m, 0)
				if err := answerNodes(m, val.(map[string]any), a); err != nil {
					return nil, err
				}
				return m, nil
			})
			if err != nil {
				return nil, fmt.Errorf("%s: %v", t.path, err)
			}
			if err := writeFile(v.Store.Root, t.path, text); err != nil {
				return nil, err
			}
			return []string{t.path}, nil
		}
		for _, p := range v.Projects {
			if r.project != "" && p.Name != r.project {
				continue
			}
			rel := p.AsksDir() + "/" + r.name + ".md"
			text, ok, err := v.Files.Read(rel)
			if err != nil {
				return nil, err
			}
			if !ok {
				if _, archived, _ := v.Files.Read(p.AsksDir() + "/archive/" + r.name + ".md"); archived {
					return nil, fmt.Errorf("ask %s/%s is already answered", p.Name, r.name)
				}
				continue
			}
			id = askRef{source: "ask", project: p.Name, name: r.name}.id()
			lines, end, err := frontmatter(text)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", rel, err)
			}
			meta, _, err := SplitFrontmatter(text)
			if err != nil {
				return nil, fmt.Errorf("%s: the frontmatter does not parse", rel)
			}
			m := mapNode(false)
			if err := answerNodes(m, meta, a); err != nil {
				return nil, err
			}
			outcome := a.Outcome
			if outcome == "" {
				outcome = "decided"
				if _, has := meta["options"]; !has {
					outcome = "done"
				}
			}
			if !contains(askOutcomes, outcome) {
				return nil, fmt.Errorf("--outcome is one of %s", strings.Join(askOutcomes, ", "))
			}
			setKey(m, "outcome", plainNode("!!str", outcome))
			add, err := encodeYAML(m)
			if err != nil {
				return nil, err
			}
			lines = append(lines[:end], append(strings.Split(strings.TrimRight(add, "\n"), "\n"), lines[end:]...)...)
			archived := p.AsksDir() + "/archive/" + r.name + ".md"
			if err := writeFile(v.Store.Root, archived, strings.Join(lines, "\n")); err != nil {
				return nil, err
			}
			if err := os.Remove(filepath.Join(v.Store.Root, rel)); err != nil {
				return nil, err
			}
			return []string{rel, archived}, nil
		}
		return nil, fmt.Errorf("no open ask %s", r.id())
	})
	return id, err
}

// DeferAsk sets an ask file's defer_until, so it leaves the queue until
// then (the operator's "later"), and pushes it. Gates have no deferral.
func (s *Store) DeferAsk(id string, until, today time.Time) (string, error) {
	r, err := parseAskRef(id)
	if err != nil {
		return "", err
	}
	if r.source != "ask" {
		return "", fmt.Errorf("only an ask file can be deferred, not %s", r.id())
	}
	if !until.After(today) {
		return "", fmt.Errorf("defer to a date after today (%s)", isoDate(today))
	}
	var ref string
	_, err = s.Publish("defer "+r.id()+" to "+isoDate(until), func(v *View) ([]string, error) {
		for _, p := range v.Projects {
			if r.project != "" && p.Name != r.project {
				continue
			}
			rel := p.AsksDir() + "/" + r.name + ".md"
			text, ok, err := v.Files.Read(rel)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			ref = askRef{source: "ask", project: p.Name, name: r.name}.id()
			lines, end, err := frontmatter(text)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", rel, err)
			}
			line := "defer_until: " + isoDate(until)
			replaced := false
			for i := 1; i < end; i++ {
				if strings.HasPrefix(lines[i], "defer_until:") {
					lines[i], replaced = line, true
				}
			}
			if !replaced {
				lines = append(lines[:end], append([]string{line}, lines[end:]...)...)
			}
			if err := writeFile(v.Store.Root, rel, strings.Join(lines, "\n")); err != nil {
				return nil, err
			}
			return []string{rel}, nil
		}
		return nil, fmt.Errorf("no open ask %s", r.id())
	})
	return ref, err
}

// answerNodes sets the answer's fields on m, the node of an ask whose
// decoded fields are meta.
func answerNodes(m *yaml.Node, meta map[string]any, a AskAnswer) error {
	if truthy(get(meta, "answered")) {
		return fmt.Errorf("already answered on %s", pyStr(get(meta, "answered")))
	}
	options := asList(get(meta, "options"))
	setKey(m, "answered", dateNode(a.Date))
	switch {
	case len(options) == 0:
		text := strings.TrimSpace(a.Text)
		if text == "" {
			text = strings.TrimSpace(a.Answer)
		}
		if text == "" {
			return fmt.Errorf("this ask has no options: answer it with --text")
		}
		setKey(m, "answer", strNode(text))
	case a.Answer == "other":
		if strings.TrimSpace(a.Text) == "" {
			return fmt.Errorf("an `other` answer needs --text")
		}
		setKey(m, "answer", plainNode("!!str", "other"))
	default:
		n, err := strconv.Atoi(a.Answer)
		if err != nil || n < 0 || n >= len(options) {
			return fmt.Errorf("the answer is an option's index, 0 to %d, or other", len(options)-1)
		}
		setKey(m, "answer", plainNode("!!int", strconv.Itoa(n)))
	}
	if len(options) > 0 && strings.TrimSpace(a.Text) != "" {
		setKey(m, "answer_text", strNode(strings.TrimSpace(a.Text)))
	}
	if a.By != "" {
		setKey(m, "answered_by", strNode(a.By))
	}
	return nil
}

// UserName is git's user.name in the store, for answered_by.
func (s *Store) UserName() string {
	out, _ := s.Git.run("config", "user.name")
	return strings.TrimSpace(out)
}

var slugJunk = regexp.MustCompile(`[^a-z0-9]+`)

// askSlug is a file name for a question: its first words, lowercased.
func askSlug(q string) string {
	words := strings.Fields(strings.Trim(slugJunk.ReplaceAllString(strings.ToLower(q), " "), " "))
	var b strings.Builder
	for _, w := range words {
		if b.Len() > 0 && b.Len()+1+len(w) > 48 {
			break
		}
		if b.Len() > 0 {
			b.WriteString("-")
		}
		b.WriteString(w)
	}
	if b.Len() == 0 {
		return "ask"
	}
	return b.String()
}

// uniqueSlug is slug, or slug-2, slug-3, ... when an open or archived ask
// already has it.
func uniqueSlug(root, dir, slug string) string {
	taken := func(s string) bool {
		for _, d := range []string{dir, dir + "/archive"} {
			if _, err := os.Stat(filepath.Join(root, d, s+".md")); err == nil {
				return true
			}
		}
		return false
	}
	out := slug
	for i := 2; taken(out); i++ {
		out = slug + "-" + strconv.Itoa(i)
	}
	return out
}

// YAML nodes for what ask new and ask answer write. Strings are double
// quoted (or literal blocks when they span lines), so nothing reads as a
// boolean or a date by accident.

func strNode(s string) *yaml.Node {
	if strings.Contains(strings.TrimRight(s, "\n"), "\n") {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimRight(s, "\n") + "\n", Style: yaml.LiteralStyle}
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s, Style: yaml.DoubleQuotedStyle}
}

func plainNode(tag, v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: v}
}

func dateNode(d time.Time) *yaml.Node { return plainNode("!!timestamp", isoDate(d)) }

func keyNode(k string) *yaml.Node { return plainNode("!!str", k) }

func mapNode(flow bool, kv ...any) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if flow {
		m.Style = yaml.FlowStyle
	}
	for i := 0; i+1 < len(kv); i += 2 {
		setKey(m, kv[i].(string), kv[i+1].(*yaml.Node))
	}
	return m
}

// setKey sets key in a mapping node, in place when it is there.
func setKey(m *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = val
			return
		}
	}
	m.Content = append(m.Content, keyNode(key), val)
}

func listNode(flow bool, items ...*yaml.Node) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: items}
	if flow {
		n.Style = yaml.FlowStyle
	}
	return n
}

func encodeYAML(n *yaml.Node) (string, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return b.String(), nil
}

// shapeNodes sets n's options, recommendation, context and from on m.
func shapeNodes(m *yaml.Node, n NewAsk) {
	if n.Kind != "" {
		setKey(m, "kind", plainNode("!!str", n.Kind))
	}
	if len(n.Categories) == 1 {
		setKey(m, "category", strNode(n.Categories[0]))
	} else if len(n.Categories) > 1 {
		var cs []*yaml.Node
		for _, c := range n.Categories {
			cs = append(cs, strNode(c))
		}
		setKey(m, "category", listNode(true, cs...))
	}
	if from := fromNode(n.From); from != nil {
		setKey(m, "from", from)
	}
	var opts []*yaml.Node
	for _, o := range n.Options {
		opts = append(opts, mapNode(true, "label", strNode(o.Label), "then", strNode(o.Then)))
	}
	setKey(m, "options", listNode(false, opts...))
	setKey(m, "recommended", plainNode("!!int", strconv.Itoa(n.Recommended)))
	if c := strings.TrimSpace(n.Context); c != "" {
		setKey(m, "context", strNode(c))
	}
}

func fromNode(f AskFrom) *yaml.Node {
	var kv []any
	if f.Thread != "" {
		kv = append(kv, "thread", strNode(f.Thread))
	}
	if f.PR != "" {
		kv = append(kv, "pr", strNode(f.PR))
	}
	if f.Dispatch != "" {
		kv = append(kv, "dispatch", strNode(f.Dispatch))
	}
	if kv == nil {
		return nil
	}
	return mapNode(true, kv...)
}

// askFileText is a new ask file: the frontmatter, then the context and the
// options for whoever reads the file itself.
func askFileText(n NewAsk) (string, error) {
	m := mapNode(false, "ask", strNode(n.Question))
	shapeNodes(m, n)
	setKey(m, "raised", dateNode(n.Raised))
	fm, err := encodeYAML(m)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("---\n" + fm + "---\n\n")
	if c := strings.TrimSpace(n.Context); c != "" {
		b.WriteString(c + "\n\n")
	}
	for i, o := range n.Options {
		rec := ""
		if i == n.Recommended {
			rec = " (recommended)"
		}
		fmt.Fprintf(&b, "%d. **%s**%s: %s\n", i, o.Label, rec, o.Then)
	}
	b.WriteString("\nAnswer with `ordvik planning ask answer <id> <n|other>`.\n")
	return b.String(), nil
}

// frontmatter splits text into its lines and the index of the closing
// `---`.
func frontmatter(text string) ([]string, int, error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \r") != "---" {
		return nil, 0, fmt.Errorf("no frontmatter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \r") == "---" {
			return lines, i, nil
		}
	}
	return nil, 0, fmt.Errorf("the frontmatter is not closed")
}

// editListItem rewrites one item of the top-level list key in text's
// frontmatter, keeping every other line as written. match picks the item
// from its decoded value; edit changes its node (or, with no match, gets
// nil and returns the item to append). The item is written back in block
// style.
func editListItem(text, key string, match func(v any) bool, edit func(n *yaml.Node) (*yaml.Node, error)) (string, error) {
	lines, end, err := frontmatter(text)
	if err != nil {
		return "", err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &doc); err != nil {
		return "", fmt.Errorf("the frontmatter does not parse: %v", yamlErrorText(err))
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("the frontmatter is not a mapping")
	}
	root := doc.Content[0]
	var seq *yaml.Node
	nextKey := end // the line after the list: the next top-level key, or the closing ---
	for i := 0; i+1 < len(root.Content); i += 2 {
		if seq != nil {
			nextKey = root.Content[i].Line
			break
		}
		if root.Content[i].Value == key {
			seq = root.Content[i+1]
			if seq.Kind != yaml.SequenceNode && !(seq.Kind == yaml.ScalarNode && seq.Tag == "!!null") {
				return "", fmt.Errorf("%s: is not a list", key)
			}
			if seq.Kind == yaml.SequenceNode && seq.Style&yaml.FlowStyle != 0 && len(seq.Content) > 0 {
				return "", fmt.Errorf("%s: is written inline; make it a block list", key)
			}
		}
	}
	at, stop := -1, -1 // the item's lines, as indices into lines
	var item *yaml.Node
	indent := "  "
	if seq != nil && seq.Kind == yaml.SequenceNode {
		for i, it := range seq.Content {
			v, err := nodeValue(it, 0)
			if err != nil {
				return "", err
			}
			if !match(v) {
				continue
			}
			at = it.Line
			stop = nextKey
			if i+1 < len(seq.Content) {
				stop = seq.Content[i+1].Line
			}
			if item, err = edit(it); err != nil {
				return "", err
			}
			break
		}
		if len(seq.Content) > 0 {
			first := lines[seq.Content[0].Line]
			indent = first[:len(first)-len(strings.TrimLeft(first, " "))]
		}
	}
	if at < 0 {
		if item, err = edit(nil); err != nil {
			return "", err
		}
	}
	out, err := encodeYAML(listNode(false, blockStyle(item)))
	if err != nil {
		return "", err
	}
	var rendered []string
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		rendered = append(rendered, indent+l)
	}
	switch {
	case at >= 0:
		for stop > at+1 && (strings.TrimSpace(lines[stop-1]) == "" || strings.HasPrefix(lines[stop-1], "#")) {
			stop--
		}
	case seq != nil && seq.Kind == yaml.SequenceNode && len(seq.Content) > 0:
		at = nextKey
		for at > 1 && (strings.TrimSpace(lines[at-1]) == "" || strings.HasPrefix(lines[at-1], "#")) {
			at--
		}
		stop = at
	case seq != nil:
		// key: [] or key: with nothing: the key's own line becomes the list.
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value == key {
				at = root.Content[i].Line
			}
		}
		stop = at + 1
		rendered = append([]string{key + ":"}, rendered...)
	default:
		at, stop = end, end
		rendered = append([]string{key + ":"}, rendered...)
	}
	lines = append(lines[:at], append(rendered, lines[stop:]...)...)
	return strings.Join(lines, "\n"), nil
}

// blockStyle writes a mapping in block style, keeping its values' styles.
func blockStyle(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.MappingNode {
		n.Style &^= yaml.FlowStyle
		for i := 1; i < len(n.Content); i += 2 {
			if v := n.Content[i]; v.Kind == yaml.SequenceNode && v.Style&yaml.FlowStyle != 0 && len(v.Content) > 0 && v.Content[0].Kind == yaml.MappingNode {
				v.Style &^= yaml.FlowStyle
			}
		}
	}
	return n
}

// gateMapping is a dispatch gate's node as a mapping: a string entry
// becomes {ask: <the string>}.
func gateMapping(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.MappingNode {
		return n
	}
	return mapNode(false, "ask", n)
}

func dispatchGateMatch(task string) func(v any) bool {
	return func(v any) bool {
		text, _ := gateFields(v)
		m := gateTask.FindStringSubmatch(text)
		return m != nil && strings.ToUpper(m[1]) == task
	}
}

func planGateMatch(id string) func(v any) bool {
	return func(v any) bool {
		m, _ := v.(map[string]any)
		return m != nil && pyStr(get(m, "id")) == id
	}
}

// gateTarget is the file and list item an ask on a gate lives in.
type gateTarget struct {
	path  string
	text  string
	key   string // human_gates or gates
	match func(v any) bool
}

func (v *View) gateTarget(r askRef) (*gateTarget, error) {
	switch r.source {
	case "dispatch":
		found, err := v.AllDocs(r.project)
		if err != nil {
			return nil, err
		}
		d := IndexDocs(found).Get(r.project + "/" + r.name)
		if d == nil || d.Archived {
			return nil, fmt.Errorf("no dispatch doc %s/%s", r.project, r.name)
		}
		if _, ok := taskMarks(d.Body)[r.sub]; !ok {
			return nil, fmt.Errorf("%s has no task %s", d.Path, r.sub)
		}
		return &gateTarget{path: d.Path, text: d.Text, key: "human_gates", match: dispatchGateMatch(r.sub)}, nil
	case "plan":
		p, _, err := v.ResolvePlan(r.project + "/" + r.name)
		if err != nil {
			return nil, err
		}
		if filepath.IsAbs(p) {
			if p, err = filepath.Rel(v.Store.Root, p); err != nil {
				return nil, err
			}
			p = filepath.ToSlash(p)
		}
		text, ok, err := v.Files.Read(p)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("no plan %s", p)
		}
		return &gateTarget{path: p, text: text, key: "gates", match: planGateMatch(r.sub)}, nil
	}
	return nil, fmt.Errorf("not a gate")
}

func writeFile(root, rel, text string) error {
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(text), 0o644)
}

// gateRef reads --gate: <project>/<dispatch>:<task> or
// plan:<project>/<plan>:<gate>.
func gateRef(gate string) (askRef, error) {
	if !strings.HasPrefix(gate, "plan:") && !strings.HasPrefix(gate, "dispatch:") {
		gate = "dispatch:" + gate
	}
	r, err := parseAskRef(gate)
	if err != nil {
		return r, fmt.Errorf("--gate takes <project>/<dispatch>:<task> or plan:<project>/<plan>:<gate>")
	}
	return r, nil
}
