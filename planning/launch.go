package planning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// A launch request and messages let a thread with no route to the
// workstation that runs dispatches start a run and nudge it, through git:
// it pushes `launch:` or a `messages:` entry onto the dispatch doc, and
// whatever drives runs acts on it (one tap on an ask, or at once for a
// kind planning.yml lists in `auto_launch`), clears the request when it
// launches and marks each message delivered once the run has it.
//
//	launch: {harness: claude, requested_by: conductor, requested_at: "2026-11-03T20:00:00Z"}
//	messages:
//	  - {id: m1, text: "Rebase on main first.", from: conductor, at: "2026-11-03T20:05:00Z"}
//	  - {id: m2, text: "...", from: ken, at: "...", delivered_at: "2026-11-03T20:31:00Z"}

// LaunchRequest is a dispatch doc's `launch:`.
type LaunchRequest struct {
	// Harness is the harness to run on; "" is the doc's own.
	Harness     string `json:"harness,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
	RequestedAt string `json:"requested_at,omitempty"`
}

// Message is one entry of a dispatch doc's `messages:`: a line for the
// doc's run, delivered at its next turn end.
type Message struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	From        string `json:"from,omitempty"`
	At          string `json:"at,omitempty"`
	DeliveredAt string `json:"delivered_at,omitempty"`
}

// MaxMessageText is the longest message text: one line a run reads as
// its next user message.
const MaxMessageText = 2000

// Pending is the messages not yet delivered, in order.
func (d *Doc) Pending() []Message {
	var out []Message
	for _, m := range d.Messages {
		if m.DeliveredAt == "" {
			out = append(out, m)
		}
	}
	return out
}

// parseLaunch reads `launch:` and `messages:`; errs is what is wrong with
// either.
func parseLaunch(meta map[string]any) (l *LaunchRequest, msgs []Message, errs []string) {
	if v := get(meta, "launch"); v != nil {
		m, ok := v.(map[string]any)
		if !ok {
			errs = append(errs, "launch is not a mapping")
		} else {
			l = &LaunchRequest{
				Harness:     strings.ToLower(strings.TrimSpace(strOf(get(m, "harness")))),
				RequestedBy: strings.TrimSpace(strOf(get(m, "requested_by"))),
				RequestedAt: strings.TrimSpace(strOf(get(m, "requested_at"))),
			}
		}
	}
	v := get(meta, "messages")
	if v == nil {
		return l, nil, errs
	}
	list, ok := v.([]any)
	if !ok {
		return l, nil, append(errs, "messages is not a list")
	}
	seen := map[string]bool{}
	for i, e := range list {
		m, ok := e.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Sprintf("messages[%d] is not a mapping", i))
			continue
		}
		msg := Message{
			ID:          strings.TrimSpace(strOf(get(m, "id"))),
			Text:        strOf(get(m, "text")),
			From:        strings.TrimSpace(strOf(get(m, "from"))),
			At:          strings.TrimSpace(strOf(get(m, "at"))),
			DeliveredAt: strings.TrimSpace(strOf(get(m, "delivered_at"))),
		}
		switch {
		case msg.ID == "":
			errs = append(errs, fmt.Sprintf("messages[%d] has no id", i))
			continue
		case seen[msg.ID]:
			errs = append(errs, fmt.Sprintf("messages[%d]: id %s is used twice", i, msg.ID))
		case strings.TrimSpace(msg.Text) == "":
			errs = append(errs, fmt.Sprintf("messages[%d] (%s) has no text", i, msg.ID))
		}
		seen[msg.ID] = true
		msgs = append(msgs, msg)
	}
	return l, msgs, errs
}

// CheckLaunches is an error for each active doc whose `launch:` or
// `messages:` is malformed, whose launch names an unknown harness, or
// that asks to launch a doc no agent may run: `executor: human`, done or
// superseded.
func CheckLaunches(found []*Doc) []string {
	var errors []string
	for _, d := range found {
		if d.Archived {
			continue
		}
		for _, e := range d.LaunchErrors {
			errors = append(errors, fmt.Sprintf("%s: %s", d.Path, e))
		}
		if d.Launch == nil {
			continue
		}
		if h := d.Launch.Harness; h != "" && !slices.Contains(Harnesses, h) {
			errors = append(errors, fmt.Sprintf("%s: launch harness %q is not one of %s", d.Path, h, strings.Join(Harnesses, ", ")))
		}
		if why := d.NotLaunchable(); why != "" {
			errors = append(errors, fmt.Sprintf("%s: launch requested, but %s", d.Path, why))
		}
	}
	return errors
}

// NotLaunchable says why no agent may run the doc, or "".
func (d *Doc) NotLaunchable() string {
	switch {
	case d.Executor == "human":
		return "the doc is executor: human"
	case d.Status == "done" || d.Status == "superseded" || d.Status == "abandoned":
		return "the doc is " + d.Status
	case d.Archived:
		return "the doc is archived"
	}
	return ""
}

// SetLaunch writes l as the frontmatter's `launch:`, or removes it when l
// is nil.
func SetLaunch(text string, l *LaunchRequest) (string, error) {
	var block []string
	if l != nil {
		var parts []string
		if l.Harness != "" {
			parts = append(parts, "harness: "+yamlString(l.Harness))
		}
		if l.RequestedBy != "" {
			parts = append(parts, "requested_by: "+yamlString(l.RequestedBy))
		}
		if l.RequestedAt != "" {
			parts = append(parts, "requested_at: "+yamlString(l.RequestedAt))
		}
		block = []string{"launch: {" + strings.Join(parts, ", ") + "}"}
	}
	return replaceKey(text, "launch", block)
}

// AddMessage appends a message to the frontmatter's `messages:`, with the
// next free id (m1, m2, ...) unless m has one; it returns the message as
// written.
func AddMessage(text string, m Message) (string, Message, error) {
	m.Text = strings.TrimSpace(m.Text)
	if err := CheckMessageText(m.Text); err != nil {
		return "", m, err
	}
	msgs, err := messagesOf(text)
	if err != nil {
		return "", m, err
	}
	if m.ID == "" {
		n := 0
		for _, old := range msgs {
			if k, err := strconv.Atoi(strings.TrimPrefix(old.ID, "m")); err == nil && strings.HasPrefix(old.ID, "m") && k > n {
				n = k
			}
		}
		m.ID = fmt.Sprintf("m%d", n+1)
	}
	if slices.ContainsFunc(msgs, func(o Message) bool { return o.ID == m.ID }) {
		return "", m, fmt.Errorf("the doc already has a message %s", m.ID)
	}
	out, err := writeMessages(text, append(msgs, m))
	return out, m, err
}

// MarkDelivered sets delivered_at on message id; changed is false when it
// was already set.
func MarkDelivered(text, id, at string) (out string, changed bool, err error) {
	msgs, err := messagesOf(text)
	if err != nil {
		return "", false, err
	}
	i := slices.IndexFunc(msgs, func(m Message) bool { return m.ID == id })
	if i < 0 {
		return "", false, fmt.Errorf("the doc has no message %s", id)
	}
	if msgs[i].DeliveredAt != "" {
		return text, false, nil
	}
	msgs[i].DeliveredAt = at
	out, err = writeMessages(text, msgs)
	return out, err == nil, err
}

// CheckMessageText is what is wrong with a message's text: it is one
// line of plain text a run reads as its next message.
func CheckMessageText(t string) error {
	switch {
	case strings.TrimSpace(t) == "":
		return fmt.Errorf("a message needs text")
	case len(t) > MaxMessageText:
		return fmt.Errorf("a message is at most %d bytes", MaxMessageText)
	}
	for _, r := range t {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("a message is one line of text, with no control characters")
		}
	}
	return nil
}

func messagesOf(text string) ([]Message, error) {
	meta, _, err := SplitFrontmatter(text)
	if err != nil {
		return nil, fmt.Errorf("the frontmatter does not parse: %v", yamlErrorText(err))
	}
	_, msgs, errs := parseLaunch(meta)
	for _, e := range errs {
		if strings.HasPrefix(e, "messages") {
			return nil, fmt.Errorf("%s; fix it by hand", e)
		}
	}
	return msgs, nil
}

func writeMessages(text string, msgs []Message) (string, error) {
	block := []string{"messages:"}
	for _, m := range msgs {
		parts := []string{"id: " + yamlString(m.ID), "text: " + yamlString(m.Text)}
		if m.From != "" {
			parts = append(parts, "from: "+yamlString(m.From))
		}
		if m.At != "" {
			parts = append(parts, "at: "+yamlString(m.At))
		}
		if m.DeliveredAt != "" {
			parts = append(parts, "delivered_at: "+yamlString(m.DeliveredAt))
		}
		block = append(block, "  - {"+strings.Join(parts, ", ")+"}")
	}
	return replaceKey(text, "messages", block)
}

// yamlString is s as a YAML double-quoted scalar (JSON's escapes are
// YAML's).
func yamlString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// replaceKey replaces the top-level frontmatter key's block (its line and
// the indented or list lines under it) with block, in place, or adds
// block at the end of the frontmatter; an empty block removes the key.
func replaceKey(text, key string, block []string) (string, error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], " \r") != "---" {
		return "", fmt.Errorf("no frontmatter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \r") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return "", fmt.Errorf("the frontmatter is not closed")
	}
	keyLine := regexp.MustCompile(`^` + regexp.QuoteMeta(key) + `:(\s|$)`)
	from, to := end, end
	for i := 1; i < end; i++ {
		if keyLine.MatchString(lines[i]) {
			from, to = i, i+1
			for to < end && (strings.HasPrefix(lines[to], " ") || strings.HasPrefix(lines[to], "-") || strings.TrimSpace(lines[to]) == "") {
				to++
			}
			break
		}
	}
	out := append(append(slices.Clone(lines[:from]), block...), lines[to:]...)
	return strings.Join(out, "\n"), nil
}

// RequestLaunch records a launch request on the dispatch doc key and
// pushes it. It refuses a doc no agent may run, and replaces an earlier
// request (the newest wins; it still launches once).
func (s *Store) RequestLaunch(key string, l LaunchRequest, now time.Time) (path string, err error) {
	if l.Harness != "" && !slices.Contains(Harnesses, l.Harness) {
		return "", fmt.Errorf("harness %q is not one of %s", l.Harness, strings.Join(Harnesses, ", "))
	}
	if l.RequestedAt == "" {
		l.RequestedAt = now.UTC().Format(time.RFC3339)
	}
	path, _, err = s.EditDoc(key, func(d *Doc) (string, string, error) {
		if why := d.NotLaunchable(); why != "" {
			return "", "", fmt.Errorf("%s cannot launch: %s", key, why)
		}
		text, err := SetLaunch(d.Text, &l)
		return text, fmt.Sprintf("%s: request a launch", key), err
	})
	return path, err
}

// SendMessage adds a message for the dispatch doc key's run and pushes
// it.
func (s *Store) SendMessage(key string, m Message, now time.Time) (path string, sent Message, err error) {
	if m.At == "" {
		m.At = now.UTC().Format(time.RFC3339)
	}
	path, _, err = s.EditDoc(key, func(d *Doc) (string, string, error) {
		text, written, err := AddMessage(d.Text, m)
		sent = written
		return text, fmt.Sprintf("%s: message %s", key, written.ID), err
	})
	return path, sent, err
}

// EditDoc changes the dispatch doc key in the planning repository and
// pushes it: fetch, fast-forward the checkout, edit, commit, push. A push
// that loses a race drops its commit and starts again on the new tip.
// edit returns the doc's new text and the commit message; changed is
// false when the text is the same.
func (s *Store) EditDoc(key string, edit func(d *Doc) (text, message string, err error)) (path string, changed bool, err error) {
	if s.Layout != LayoutRepo {
		return "", false, fmt.Errorf("this writes to the planning repository; run it in its checkout")
	}
	if !strings.Contains(key, "/") {
		return "", false, fmt.Errorf("name the dispatch as <project>/<id>")
	}
	ref := s.Git.DefaultRef()
	if ref == "origin/HEAD" {
		if out, err := s.Git.run("rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil {
			ref = strings.TrimSpace(out)
		}
	}
	branch := strings.TrimPrefix(ref, "origin/")
	cur, err := s.Git.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(cur) != branch {
		return "", false, fmt.Errorf("the planning checkout is on %s; check out %s first", strings.TrimSpace(cur), branch)
	}
	if out, _ := s.Git.run("status", "--porcelain"); strings.TrimSpace(out) != "" {
		return "", false, fmt.Errorf("the planning checkout has uncommitted changes")
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := s.Git.Fetch(); err != nil {
			return "", false, err
		}
		if _, err := s.Git.run("merge", "--ff-only", "-q", ref); err != nil {
			return "", false, err
		}
		d, err := s.docAt(key)
		if err != nil {
			return "", false, err
		}
		text, msg, err := edit(d)
		if err != nil {
			return d.Path, false, err
		}
		if text == d.Text {
			return d.Path, false, nil
		}
		if err := os.WriteFile(filepath.Join(s.Root, d.Path), []byte(text), 0o644); err != nil {
			return d.Path, false, err
		}
		if _, err := s.Git.run("add", "--", d.Path); err != nil {
			return d.Path, false, err
		}
		if _, err := s.Git.run("commit", "-q", "-m", msg); err != nil {
			return d.Path, false, err
		}
		if _, lastErr = s.Git.run("push", "-q", "origin", "HEAD:"+branch); lastErr == nil {
			return d.Path, true, nil
		}
		// Lost a race (or the push failed): drop the commit and retry on
		// the new tip. The checkout was clean, so this drops only ours.
		if _, err := s.Git.run("reset", "-q", "--hard", ref); err != nil {
			return d.Path, false, err
		}
	}
	return "", false, lastErr
}

// EditDocInPlace changes the dispatch doc key in the working tree only,
// for a caller that commits it with other files of its own (a driver
// clearing a launch request in the commit that records the run).
func (s *Store) EditDocInPlace(key string, edit func(d *Doc) (text, message string, err error)) (path string, changed bool, err error) {
	d, err := s.docIn(key, true)
	if err != nil {
		return "", false, err
	}
	text, _, err := edit(d)
	if err != nil || text == d.Text {
		return d.Path, false, err
	}
	return d.Path, true, os.WriteFile(filepath.Join(s.Root, d.Path), []byte(text), 0o644)
}

func (s *Store) docAt(key string) (*Doc, error) { return s.docIn(key, true) }

func (s *Store) docIn(key string, worktree bool) (*Doc, error) {
	view, err := s.NewView("", worktree)
	if err != nil {
		return nil, err
	}
	project, _, _ := strings.Cut(key, "/")
	found, err := view.AllDocs(project)
	if err != nil {
		return nil, err
	}
	d := IndexDocs(found).Get(key)
	if d == nil {
		return nil, fmt.Errorf("no dispatch doc %s", key)
	}
	return d, nil
}

// AutoLaunch is planning.yml's `auto_launch`: the dispatch kinds whose
// launch requests launch without an ask.
func (v *View) AutoLaunch() []string {
	text, ok, err := v.Files.Read(repoMarker)
	if err != nil || !ok {
		return nil
	}
	val, err := decodeYAML(text)
	if err != nil {
		return nil
	}
	m, _ := val.(map[string]any)
	var kinds []string
	for _, k := range asListLoose(get(m, "auto_launch")) {
		if s := strings.ToLower(strings.TrimSpace(strOf(k))); s != "" {
			kinds = append(kinds, s)
		}
	}
	return kinds
}

// CheckAutoLaunch is an error for each auto_launch kind that is not one
// of Kinds.
func CheckAutoLaunch(kinds []string) []string {
	var errors []string
	for _, k := range kinds {
		if !slices.Contains(Kinds, k) {
			errors = append(errors, fmt.Sprintf("planning.yml: auto_launch kind %q is not one of %s", k, strings.Join(Kinds, ", ")))
		}
	}
	return errors
}
