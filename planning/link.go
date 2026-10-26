package planning

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Link records l on the dispatch doc key (<project>/<id>) in the planning
// repository and pushes it: fetch, fast-forward the checkout, add the
// `prs:` entry, commit, push. A push that loses a race drops its commit
// and starts again on the new tip. added is false when the doc already has
// the entry.
func (s *Store) Link(key string, l PRLink) (path string, added bool, err error) {
	if s.Layout != LayoutRepo {
		return "", false, fmt.Errorf("link writes to the planning repository; run it in its checkout")
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
		view, err := s.NewView("", true)
		if err != nil {
			return "", false, err
		}
		found, err := view.AllDocs("")
		if err != nil {
			return "", false, err
		}
		d := IndexDocs(found).Get(key)
		if d == nil {
			return "", false, fmt.Errorf("no dispatch doc %s", key)
		}
		text, err := AddPRLinkChecked(d, l)
		if err != nil {
			return d.Path, false, fmt.Errorf("%s: %v", d.Path, err)
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
		if _, err := s.Git.run("commit", "-q", "-m", fmt.Sprintf("%s: link %s", key, l.Ref())); err != nil {
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

// AddPRLinkChecked is AddPRLink on d's text, refusing a task the doc does
// not have.
func AddPRLinkChecked(d *Doc, l PRLink) (string, error) {
	for _, t := range l.Tasks {
		if _, ok := d.Tasks[t]; !ok {
			return "", fmt.Errorf("the doc has no task %d", t)
		}
	}
	text, _, err := AddPRLink(d.Text, l)
	return text, err
}
