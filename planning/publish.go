package planning

import (
	"fmt"
	"os"
	"strings"
)

// Publish makes one change on the tip of the planning repository's
// default branch and pushes it, the way every writer of the planning
// repository does: fetch, change, commit, push, and on a refused push
// start again on the new tip. The change is made in a temporary worktree,
// so the caller's checkout may be on any branch, dirty, or behind;
// afterwards a clean checkout of the default branch is fast-forwarded to
// what was pushed.
//
// change gets a view of the tip's working tree and returns the paths it
// wrote or removed (relative to the view's root, v.Store.Root); none
// means there is nothing to commit, and Publish returns false.
func (s *Store) Publish(message string, change func(v *View) ([]string, error)) (bool, error) {
	if s.Layout != LayoutRepo {
		return false, fmt.Errorf("this writes to the planning repository; run it in its checkout")
	}
	if err := s.Git.Fetch(); err != nil {
		return false, err
	}
	ref := s.Git.DefaultRef()
	if ref == "origin/HEAD" {
		if out, err := s.Git.run("rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil {
			ref = strings.TrimSpace(out)
		}
	}
	branch := strings.TrimPrefix(ref, "origin/")
	tmp, err := os.MkdirTemp("", "ordvik-publish-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmp)
	if _, err := s.Git.run("worktree", "add", "-q", "--detach", tmp, ref); err != nil {
		return false, err
	}
	defer s.Git.run("worktree", "remove", "--force", tmp)
	w := &Store{Root: tmp, Git: Git{Root: tmp}, Layout: LayoutRepo}

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			if err := s.Git.Fetch(); err != nil {
				return false, err
			}
			if _, err := w.Git.run("reset", "-q", "--hard", ref); err != nil {
				return false, err
			}
		}
		view, err := w.NewView("", true)
		if err != nil {
			return false, err
		}
		paths, err := change(view)
		if err != nil {
			return false, err
		}
		if len(paths) == 0 {
			return false, nil
		}
		if _, err := w.Git.run(append([]string{"add", "-A", "--"}, paths...)...); err != nil {
			return false, err
		}
		if _, err := w.Git.run("commit", "-q", "-m", message); err != nil {
			return false, err
		}
		if _, lastErr = w.Git.run("push", "-q", "origin", "HEAD:"+branch); lastErr == nil {
			s.fastForward(ref, branch)
			return true, nil
		}
		// Lost a race (or the push failed): redo the change on the new tip.
	}
	return false, lastErr
}

// fastForward brings the caller's checkout up to ref when it is a clean
// checkout of branch; anything else is left alone.
func (s *Store) fastForward(ref, branch string) {
	s.Git.Fetch()
	cur, err := s.Git.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || strings.TrimSpace(cur) != branch {
		return
	}
	if out, _ := s.Git.run("status", "--porcelain"); strings.TrimSpace(out) != "" {
		return
	}
	s.Git.run("merge", "--ff-only", "-q", ref)
}
