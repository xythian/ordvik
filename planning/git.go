package planning

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Git runs git in one repository.
type Git struct {
	Root string
}

func (g Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", g.Root}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// RepoRoot is the top of the git working tree that holds dir.
func RepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// Fetch fetches origin quietly.
func (g Git) Fetch() error {
	_, err := g.run("fetch", "-q", "origin")
	return err
}

// DefaultRef is the remote's default branch: origin/HEAD, else
// origin/main or origin/master.
func (g Git) DefaultRef() string {
	for _, ref := range []string{"origin/HEAD", "origin/main", "origin/master"} {
		if _, err := g.run("rev-parse", "--verify", "-q", ref); err == nil {
			return ref
		}
	}
	if _, err := g.run("rev-parse", "--verify", "-q", "origin"); err != nil {
		if _, err := g.run("remote", "get-url", "origin"); err != nil {
			return "HEAD" // a clone with no remote, such as a fresh import
		}
	}
	return "origin/master"
}

// Commit is the commit ref names.
func (g Git) Commit(ref string) (string, error) {
	out, err := g.run("rev-parse", "--verify", ref+"^{commit}")
	return strings.TrimSpace(out), err
}

// RemoteBranches is the set of branch names under refs/remotes/origin.
func (g Git) RemoteBranches() map[string]bool {
	out, _ := g.run("for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	set := map[string]bool{}
	for _, b := range strings.Fields(out) {
		if strings.HasPrefix(b, "origin/") {
			set[strings.TrimPrefix(b, "origin/")] = true
		}
	}
	return set
}

// Shallow says whether the clone is shallow.
func (g Git) Shallow() bool {
	out, _ := g.run("rev-parse", "--is-shallow-repository")
	return strings.TrimSpace(out) == "true"
}

// PathDates is {path: date} from one pass over git log: the newest commit
// touching each path, or with added the commit that added it.
func (g Git) PathDates(ref, pathspec string, added bool) map[string]time.Time {
	args := []string{"log", "--format=@%as", "--name-only"}
	if added {
		args = append(args, "--diff-filter=A")
	}
	args = append(args, ref, "--", pathspec)
	out, _ := g.run(args...)
	dates := map[string]time.Time{}
	var date time.Time
	have := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "@") {
			d, ok := parseDate(line[1:])
			date, have = d, ok
			continue
		}
		if line == "" || !have {
			continue
		}
		if added {
			dates[line] = date // oldest wins: log runs newest first
		} else if _, set := dates[line]; !set {
			dates[line] = date
		}
	}
	return dates
}

// Tree is a set of files to read: a commit, or the working tree.
type Tree interface {
	// Files lists the files under dir (recursively), sorted as git does.
	Files(dir string) ([]string, error)
	// Read returns a file's text; ok is false when it does not exist.
	Read(path string) (text string, ok bool, err error)
	// Name says where the files come from, for messages.
	Name() string
}

// RefTree reads files from a commit, with one `git cat-file --batch`.
type RefTree struct {
	git   Git
	ref   string
	cache map[string]*string
}

func NewRefTree(g Git, ref string) *RefTree {
	return &RefTree{git: g, ref: ref, cache: map[string]*string{}}
}

func (t *RefTree) Name() string { return t.ref }

func (t *RefTree) Files(dir string) ([]string, error) {
	dir = strings.TrimSuffix(dir, "/")
	args := []string{"ls-tree", "-r", "--name-only", "-z", t.ref}
	if dir != "" {
		args = append(args, "--", dir+"/")
	}
	out, err := t.git.run(args...)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	return files, nil
}

// Prefetch reads many files in one git process.
func (t *RefTree) Prefetch(paths []string) error {
	var want []string
	for _, p := range paths {
		if _, ok := t.cache[p]; !ok {
			want = append(want, p)
		}
	}
	if len(want) == 0 {
		return nil
	}
	cmd := exec.Command("git", "-C", t.git.Root, "cat-file", "--batch")
	var in bytes.Buffer
	for _, p := range want {
		in.WriteString(t.ref + ":" + p + "\n")
	}
	cmd.Stdin = &in
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git cat-file --batch: %v", err)
	}
	r := bufio.NewReader(bytes.NewReader(out))
	for _, p := range want {
		header, err := r.ReadString('\n')
		if err != nil {
			return fmt.Errorf("git cat-file --batch: %v", err)
		}
		fields := strings.Fields(header)
		if len(fields) == 2 && fields[1] == "missing" {
			t.cache[p] = nil
			continue
		}
		if len(fields) != 3 {
			return fmt.Errorf("git cat-file --batch: unexpected %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return fmt.Errorf("git cat-file --batch: unexpected %q", header)
		}
		buf := make([]byte, size+1) // the object, then a newline
		if _, err := io.ReadFull(r, buf); err != nil {
			return fmt.Errorf("git cat-file --batch: %v", err)
		}
		if fields[1] != "blob" {
			t.cache[p] = nil
			continue
		}
		s := string(buf[:size])
		t.cache[p] = &s
	}
	return nil
}

func (t *RefTree) Read(path string) (string, bool, error) {
	if err := t.Prefetch([]string{path}); err != nil {
		return "", false, err
	}
	s := t.cache[path]
	if s == nil {
		return "", false, nil
	}
	return *s, true, nil
}

// FSTree reads files from a working tree.
type FSTree struct {
	Root string
}

func (t FSTree) Name() string { return "the working tree" }

func (t FSTree) Files(dir string) ([]string, error) {
	base := filepath.Join(t.Root, dir)
	var files []string
	err := filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(t.Root, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)
	return files, err
}

func (t FSTree) Read(path string) (string, bool, error) {
	b, err := os.ReadFile(filepath.Join(t.Root, path))
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}

// listDir is the sorted names of plain files directly in dir of the
// working tree (os.listdir in the scripts).
func listDir(root, dir string) []string {
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02", "20060102"} {
		if d, err := time.Parse(layout, s); err == nil {
			return d, true
		}
	}
	return time.Time{}, false
}

func isoDate(d time.Time) string { return d.Format("2006-01-02") }
