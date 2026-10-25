package planning

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	taskHeading = regexp.MustCompile(`(?i)^##\s+Task\s+(\d+)\b(.*)$`)
	doneMark    = regexp.MustCompile(`[—-]\s*DONE\s*$`)
)

// SplitFrontmatter is (frontmatter, body) of a markdown doc; (nil, text)
// without one. The error is set when the frontmatter is not valid YAML.
func SplitFrontmatter(text string) (map[string]any, string, error) {
	if strings.HasPrefix(text, "---\n") {
		if end := strings.Index(text[4:], "\n---\n"); end != -1 {
			end += 4
			val, err := decodeYAML(text[4:end])
			if err != nil {
				return nil, text, err
			}
			m, _ := val.(map[string]any)
			if m == nil {
				m = map[string]any{}
			}
			return m, text[end+5:], nil
		}
	}
	return map[string]any{}, text, nil
}

// LooseFrontmatter is the top-level `key: value` pairs of a doc whose
// frontmatter does not parse: enough to place it while check reports the
// error.
func LooseFrontmatter(text string) (map[string]any, string) {
	end := strings.Index(text[4:], "\n---\n") + 4
	meta := map[string]any{}
	for _, line := range strings.Split(text[4:end], "\n") {
		key, value, sep := strings.Cut(line, ":")
		if sep && key != "" && !isSpace(key[0]) && !strings.HasPrefix(key, "-") {
			v := strings.Trim(strings.TrimSpace(value), `"`)
			if v == "" {
				meta[strings.TrimSpace(key)] = nil
			} else {
				meta[strings.TrimSpace(key)] = v
			}
		}
	}
	return meta, text[end+5:]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == '\v'
}

func yamlErrorText(err error) string {
	return strings.ReplaceAll(err.Error(), "\n", " ")
}

// Doc is a dispatch doc.
type Doc struct {
	Path      string
	Project   string
	ID        string // bare id, unique within the project
	Thread    string // the thread it belongs to (planning repository only)
	Repo      string
	Branch    any
	Executor  string
	Status    string
	Plan      any
	Wave      any
	DependsOn []string
	Title     *string
	Tasks     map[int]bool
	TaskOrder []int
	Archived  bool
	YAMLError string
	Meta      map[string]any
	Body      string
	Text      string
}

// Key is the doc's qualified id.
func (d *Doc) Key() string { return d.Project + "/" + d.ID }

// TasksDone is how many task headings are marked — DONE.
func (d *Doc) TasksDone() int {
	n := 0
	for _, done := range d.Tasks {
		if done {
			n++
		}
	}
	return n
}

// BranchName is the branch as a string, "" when unset.
func (d *Doc) BranchName() string {
	if d.Branch == nil {
		return ""
	}
	return pyStr(d.Branch)
}

// ParseDoc parses a dispatch doc. layout decides which field holds its id:
// `thread:` in the legacy layout, `id:` in the planning repository (where
// `thread:` names the doc's thread).
func ParseDoc(layout, project, p, text string) *Doc {
	meta, body, err := SplitFrontmatter(text)
	d := &Doc{Path: p, Project: project, Tasks: map[int]bool{}, Text: text}
	if err != nil {
		d.YAMLError = yamlErrorText(err)
		meta, body = LooseFrontmatter(text)
	}
	d.Meta, d.Body = meta, body
	for _, line := range strings.Split(body, "\n") {
		if d.Title == nil && strings.HasPrefix(line, "# ") {
			t := strings.TrimSpace(line[2:])
			d.Title = &t
		}
		if m := taskHeading.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			if _, seen := d.Tasks[n]; !seen {
				d.TaskOrder = append(d.TaskOrder, n)
			}
			d.Tasks[n] = doneMark.MatchString(m[2])
		}
	}
	if layout == LayoutRepo {
		if _, ok := meta["id"]; ok {
			d.ID = strings.TrimSpace(pyStr(meta["id"]))
			if t := get(meta, "thread"); t != nil {
				d.Thread = pyStr(t)
			}
		} else {
			d.ID = strings.TrimSpace(pyStr(getOr(meta, "thread", "")))
		}
		d.Repo = repoName(get(meta, "repo"))
	} else {
		d.ID = strings.TrimSpace(pyStr(getOr(meta, "thread", "")))
	}
	d.Branch = get(meta, "branch")
	d.Executor = strings.ToLower(pyStr(getOr(meta, "executor", "")))
	d.Status = strings.ToLower(pyStr(orDefault(get(meta, "status"), "active")))
	d.Plan = get(meta, "plan")
	d.Wave = get(meta, "wave")
	for _, dep := range asListLoose(get(meta, "depends_on")) {
		d.DependsOn = append(d.DependsOn, pyStr(dep))
	}
	d.Archived = strings.Contains(p, "/archive/")
	return d
}

var repoURL = regexp.MustCompile(`github\.com[:/]([^/\s]+/[^/\s]+?)(?:\.git)?/?$`)

// repoName is a dispatch doc's `repo:` as owner/name. The planning repository writes it
// so; the older form is a mapping with the clone `url` and a `path`.
func repoName(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case map[string]any:
		if m := repoURL.FindStringSubmatch(pyStr(get(x, "url"))); m != nil {
			return m[1]
		}
		if p := get(x, "path"); p != nil {
			s := strings.TrimRight(pyStr(p), "/")
			return s[strings.LastIndex(s, "/")+1:]
		}
		return ""
	}
	return pyStr(v)
}

// asListLoose is scripts/plan's as_list: a list, or a string holding a
// YAML list.
func asListLoose(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case string:
		val, err := decodeYAML(x)
		if err != nil {
			return nil
		}
		l, _ := val.([]any)
		return l
	}
	return nil
}

// AllDocs is every dispatch doc that names an id, active and archived, in
// every project (or only in the named one).
func (v *View) AllDocs(only string) ([]*Doc, error) {
	var found []*Doc
	for _, p := range v.Projects {
		if only != "" && p.Name != only {
			continue
		}
		dir := p.DispatchDir()
		files, err := v.Docs.Files(dir)
		if err != nil {
			return nil, err
		}
		var want []string
		for _, f := range files {
			if !strings.HasSuffix(f, ".md") {
				continue
			}
			rel := f[len(dir)+1:]
			if strings.Contains(rel, "/") && !strings.HasPrefix(rel, "archive/") {
				continue
			}
			want = append(want, f)
		}
		if rt, ok := v.Docs.(*RefTree); ok {
			if err := rt.Prefetch(want); err != nil {
				return nil, err
			}
		}
		for _, f := range want {
			text, _, err := v.Docs.Read(f)
			if err != nil {
				return nil, err
			}
			d := ParseDoc(v.Store.Layout, p.Name, f, text)
			if d.ID != "" {
				found = append(found, d)
			}
		}
	}
	return found, nil
}

// Docs is dispatch docs by qualified id, in the order first seen; an
// active doc wins over an archived one.
type Docs struct {
	keys []string
	byID map[string]*Doc
}

func IndexDocs(found []*Doc) *Docs {
	docs := &Docs{byID: map[string]*Doc{}}
	for _, d := range found {
		k := d.Key()
		old, ok := docs.byID[k]
		if !ok {
			docs.keys = append(docs.keys, k)
		}
		if !ok || old.Archived {
			docs.byID[k] = d
		}
	}
	return docs
}

// Get is the doc with qualified id key, or nil.
func (d *Docs) Get(key string) *Doc {
	if d == nil {
		return nil
	}
	return d.byID[key]
}

// All is every doc, in index order.
func (d *Docs) All() []*Doc {
	out := make([]*Doc, len(d.keys))
	for i, k := range d.keys {
		out[i] = d.byID[k]
	}
	return out
}

// CheckIDs: two active docs with one id are an error; an active doc
// sharing its id with an archived one is a warning. Archived docs alone
// are history.
func CheckIDs(found []*Doc, home string) (errors, warnings []string) {
	byID := map[string][]*Doc{}
	for _, d := range found {
		byID[d.Key()] = append(byID[d.Key()], d)
	}
	ids := make([]string, 0, len(byID))
	for k := range byID {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	for _, k := range ids {
		var active, archived []string
		for _, d := range byID[k] {
			if d.Archived {
				archived = append(archived, d.Path)
			} else {
				active = append(active, d.Path)
			}
		}
		id := Unqualify(home, k)
		if len(active) > 1 {
			errors = append(errors, fmt.Sprintf("thread %s is claimed by %d dispatch docs: %s",
				id, len(active), strings.Join(active, ", ")))
		} else if len(active) > 0 && len(archived) > 0 {
			warnings = append(warnings, fmt.Sprintf("thread %s: %s shares its id with %s",
				id, active[0], strings.Join(archived, ", ")))
		}
	}
	return errors, warnings
}
