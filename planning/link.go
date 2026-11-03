package planning

import (
	"fmt"
)

// Link records l on the dispatch doc key (<project>/<id>) in the planning
// repository and pushes it: fetch, fast-forward the checkout, add the
// `prs:` entry, commit, push. A push that loses a race drops its commit
// and starts again on the new tip. added is false when the doc already has
// the entry.
func (s *Store) Link(key string, l PRLink) (path string, added bool, err error) {
	return s.EditDoc(key, func(d *Doc) (string, string, error) {
		text, err := AddPRLinkChecked(d, l)
		if err != nil {
			return "", "", fmt.Errorf("%s: %v", d.Path, err)
		}
		return text, fmt.Sprintf("%s: link %s", key, l.Ref()), nil
	})
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
