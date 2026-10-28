package planning

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A dispatch doc records the pull requests that carry its work under
// `prs:`, and done is read from them: a task
// is done once a PR that lists it has merged, and a dispatch is done once a
// PR marked `done` has merged and no human gate is open. The link lives in
// the planning repository, so code repositories, public ones included,
// carry nothing about planning in their history.
//
//	prs:
//	  - {pr: "xythian/ordvik#12", tasks: [1, 2]}
//	  - {pr: "xythian/ordvik#14", done: true}

// PRLink is one entry of a dispatch doc's `prs:`.
type PRLink struct {
	Repo   string `json:"repo"`
	Number int    `json:"number"`
	Tasks  []int  `json:"tasks,omitempty"`
	Done   bool   `json:"done,omitempty"`
}

// Ref is the PR as owner/name#n.
func (l PRLink) Ref() string { return fmt.Sprintf("%s#%d", l.Repo, l.Number) }

// Line is the entry as `link` writes it.
func (l PRLink) Line() string {
	s := fmt.Sprintf("  - {pr: %q", l.Ref())
	if len(l.Tasks) > 0 {
		ns := make([]string, len(l.Tasks))
		for i, n := range l.Tasks {
			ns[i] = strconv.Itoa(n)
		}
		s += ", tasks: [" + strings.Join(ns, ", ") + "]"
	}
	if l.Done {
		s += ", done: true"
	}
	return s + "}"
}

var prRef = regexp.MustCompile(`^([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#([1-9][0-9]*)$`)

// ParsePRRef parses owner/name#n.
func ParsePRRef(s string) (string, int, bool) {
	m := prRef.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", 0, false
	}
	n, _ := strconv.Atoi(m[2])
	return m[1], n, true
}

// parsePRLinks reads `prs:`, with an error for each entry it cannot read.
func parsePRLinks(v any) ([]PRLink, []string) {
	var links []PRLink
	var errs []string
	for i, e := range asListLoose(v) {
		m, ok := e.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("prs[%d] is not a mapping with pr:", i))
			continue
		}
		repo, n, ok := ParsePRRef(pyStr(get(m, "pr")))
		if !ok {
			errs = append(errs, fmt.Sprintf("prs[%d]: pr %q is not owner/name#number", i, pyStr(get(m, "pr"))))
			continue
		}
		l := PRLink{Repo: repo, Number: n, Done: truthy(get(m, "done"))}
		for _, t := range asListLoose(get(m, "tasks")) {
			k, err := strconv.Atoi(pyStr(t))
			if err != nil {
				errs = append(errs, fmt.Sprintf("prs[%d]: task %q is not a number", i, pyStr(t)))
				continue
			}
			l.Tasks = append(l.Tasks, k)
		}
		links = append(links, l)
	}
	if v != nil && links == nil && errs == nil {
		if _, ok := v.([]any); !ok {
			errs = append(errs, "prs is not a list")
		}
	}
	return links, errs
}

// CheckPRs is an error for each `prs:` entry that cannot be read, and for
// a task it names that the doc does not have.
func CheckPRs(found []*Doc) []string {
	var errors []string
	for _, d := range found {
		for _, e := range d.PRErrors {
			errors = append(errors, fmt.Sprintf("%s: %s", d.Path, e))
		}
		for _, l := range d.PRs {
			for _, t := range l.Tasks {
				if _, ok := d.Tasks[t]; !ok {
					errors = append(errors, fmt.Sprintf("%s: %s names task %d, which the doc does not have", d.Path, l.Ref(), t))
				}
			}
		}
	}
	return errors
}

// PRSource says whether a pull request has merged.
type PRSource interface {
	Merged(repo string, number int) (bool, error)
}

// GitHub reads PR state from the GitHub API with $GITHUB_TOKEN or
// $GH_TOKEN, else through the gh CLI. Answers are cached for the run.
type GitHub struct {
	cache map[string]bool
}

func (g *GitHub) Merged(repo string, number int) (bool, error) {
	key := fmt.Sprintf("%s#%d", repo, number)
	if m, ok := g.cache[key]; ok {
		return m, nil
	}
	path := fmt.Sprintf("repos/%s/pulls/%d", repo, number)
	var body []byte
	var err error
	if tok := firstEnv("GITHUB_TOKEN", "GH_TOKEN"); tok != "" {
		body, err = apiGet(path, tok)
	} else if _, lerr := exec.LookPath("gh"); lerr == nil {
		body, err = exec.Command("gh", "api", path).Output()
	} else {
		err = fmt.Errorf("no GITHUB_TOKEN and no gh")
	}
	if err != nil {
		return false, err
	}
	var pr struct {
		Merged bool `json:"merged"`
	}
	if err := json.Unmarshal(body, &pr); err != nil {
		return false, err
	}
	if g.cache == nil {
		g.cache = map[string]bool{}
	}
	g.cache[key] = pr.Merged
	return pr.Merged, nil
}

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

func apiGet(path, token string) ([]byte, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// DerivePRs marks tasks and docs done from their merged PRs. A PR marked
// done completes every task but a human gate's, and the doc only when
// every gate's task is marked; otherwise the doc stays active and the
// returned notes say its code merged with a gate open. A PR whose state
// cannot be read changes nothing and gets a note.
func DerivePRs(found []*Doc, src PRSource) []string {
	var notes []string
	unknown := map[string]bool{}
	for _, d := range found {
		if d.Archived || d.Status != "active" || len(d.PRs) == 0 {
			continue
		}
		gates := d.gateTasks()
		doneMerged := false
		for _, l := range d.PRs {
			merged, err := src.Merged(l.Repo, l.Number)
			if err != nil {
				if !unknown[l.Repo] {
					notes = append(notes, fmt.Sprintf("PR state unknown for %s (%v); its dispatch docs show only their marks", l.Repo, err))
					unknown[l.Repo] = true
				}
				continue
			}
			if !merged {
				continue
			}
			for _, t := range l.Tasks {
				if _, ok := d.Tasks[t]; ok && !gates[t] {
					d.Tasks[t] = true
				}
			}
			if l.Done {
				doneMerged = true
				for t := range d.Tasks {
					if !gates[t] {
						d.Tasks[t] = true
					}
				}
			}
		}
		if !doneMerged {
			continue
		}
		gateOpen := d.openUntaskedGate()
		for t := range gates {
			gateOpen = gateOpen || !d.Tasks[t]
		}
		if gateOpen {
			notes = append(notes, fmt.Sprintf("%s: merged, gate open", d.Key()))
			continue
		}
		d.Status = "done"
		d.DoneBy = "PR"
	}
	return notes
}

// gateTasks is the numbered tasks the doc's human gates name.
func (d *Doc) gateTasks() map[int]bool {
	out := map[int]bool{}
	for _, e := range asList(get(d.Meta, "human_gates")) {
		text, _ := gateFields(e)
		if m := gateTask.FindStringSubmatch(text); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				out[n] = true
			}
		}
	}
	return out
}

// openUntaskedGate says whether a human gate names no task, so only the
// doc's own status can close it.
func (d *Doc) openUntaskedGate() bool {
	for _, e := range asList(get(d.Meta, "human_gates")) {
		text, fields := gateFields(e)
		if gateTask.FindStringSubmatch(text) == nil && parseShape(fields).Answered == "" {
			return true
		}
	}
	return false
}

var prsKey = regexp.MustCompile(`^prs:\s*(#.*)?$`)

// AddPRLink adds a `prs:` entry to a dispatch doc's frontmatter, keeping
// every other line as written. An entry already there for the same PR with
// the same tasks is not added twice (added is false).
func AddPRLink(text string, l PRLink) (out string, added bool, err error) {
	meta, _, ferr := SplitFrontmatter(text)
	if ferr != nil {
		return "", false, fmt.Errorf("the frontmatter does not parse: %v", yamlErrorText(ferr))
	}
	existing, errs := parsePRLinks(get(meta, "prs"))
	if len(errs) > 0 {
		return "", false, fmt.Errorf("prs: %s; fix it by hand", errs[0])
	}
	for _, e := range existing {
		if e.Ref() == l.Ref() && e.Done == l.Done && fmt.Sprint(e.Tasks) == fmt.Sprint(l.Tasks) {
			return text, false, nil
		}
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \r") != "---" {
		return "", false, fmt.Errorf("no frontmatter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \r") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return "", false, fmt.Errorf("the frontmatter is not closed")
	}
	at := -1
	for i := 1; i < end; i++ {
		if prsKey.MatchString(lines[i]) {
			at = i + 1
			for at < end && (strings.HasPrefix(lines[at], " ") || strings.HasPrefix(lines[at], "-")) {
				at++
			}
			break
		}
		if strings.HasPrefix(lines[i], "prs:") {
			return "", false, fmt.Errorf("prs: is written inline; make it a block list to add to it")
		}
	}
	ins := []string{l.Line()}
	if at < 0 {
		at = end
		ins = []string{"prs:", l.Line()}
	}
	lines = append(lines[:at], append(ins, lines[at:]...)...)
	return strings.Join(lines, "\n"), true, nil
}
