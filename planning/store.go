package planning

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// A planning store is either the legacy layout, docs/planning/ inside a
// code repository read as one project, or the planning repository:
// planning.yml at its root and one directory per project under
// projects/.
const (
	LayoutLegacy = "legacy"
	LayoutRepo   = "repo"

	legacyDir    = "docs/planning"
	projectsDir  = "projects"
	repoMarker   = "planning.yml"
	projectFile  = "project.yml"
	ordvikConfig = ".ordvik.yml"
)

// Store is a planning store: a git repository and its layout.
type Store struct {
	Root   string
	Git    Git
	Layout string
}

// Dependency is a repository a project depends on.
type Dependency struct {
	Repo  string   `json:"repo"`
	Paths []string `json:"paths,omitempty"`
}

// Project is one project's directory.
type Project struct {
	Name      string       `json:"project"`
	Title     string       `json:"title"`
	Dir       string       `json:"dir"`
	Owns      []string     `json:"owns"`
	DependsOn []Dependency `json:"depends_on"`
	Incoming  string       `json:"incoming"`
}

func (p *Project) DispatchDir() string { return p.Dir + "/dispatch" }
func (p *Project) PlansDir() string    { return p.Dir + "/plans" }
func (p *Project) AsksDir() string     { return p.Dir + "/asks" }

// Open finds the store that holds dir.
func Open(dir string) (*Store, error) {
	root, err := RepoRoot(dir)
	if err != nil {
		return nil, err
	}
	s := &Store{Root: root, Git: Git{Root: root}, Layout: LayoutLegacy}
	if _, err := os.Stat(filepath.Join(root, repoMarker)); err == nil {
		s.Layout = LayoutRepo
	}
	return s, nil
}

// View is what one command reads: dispatch docs from Docs (a commit, or
// the working tree), plans and ask files from Files. In the legacy layout
// Files is the working tree, as scripts/plan and scripts/asks read it; in
// the planning repository both are the same tree.
type View struct {
	Store    *Store
	Ref      string
	Commit   string
	Docs     Tree
	Files    Tree
	Projects []*Project
}

// NewView reads dispatch docs from ref, or from the working tree when ref
// is empty and the store is the planning repository.
func (s *Store) NewView(ref string, worktree bool) (*View, error) {
	v := &View{Store: s, Ref: ref}
	if worktree {
		v.Ref = "the working tree"
		v.Docs = FSTree{Root: s.Root}
		v.Files = v.Docs
		if c, err := s.Git.Commit("HEAD"); err == nil {
			v.Commit = c
		}
	} else {
		if v.Ref == "" {
			v.Ref = s.Git.DefaultRef()
		}
		c, err := s.Git.Commit(v.Ref)
		if err != nil {
			return nil, err
		}
		v.Commit = c
		v.Docs = NewRefTree(s.Git, v.Ref)
		v.Files = FSTree{Root: s.Root}
		if s.Layout == LayoutRepo {
			v.Files = v.Docs
		}
	}
	projects, err := s.loadProjects(v.Files)
	if err != nil {
		return nil, err
	}
	v.Projects = projects
	return v, nil
}

var projectName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func (s *Store) loadProjects(t Tree) ([]*Project, error) {
	if s.Layout == LayoutLegacy {
		p := &Project{Name: legacyProjectName(s.Root), Dir: legacyDir, Incoming: "open"}
		p.Title = p.Name
		return []*Project{p}, nil
	}
	files, err := t.Files(projectsDir)
	if err != nil {
		return nil, err
	}
	var out []*Project
	for _, f := range files {
		parts := strings.Split(f, "/")
		if len(parts) != 3 || parts[2] != projectFile {
			continue
		}
		text, _, err := t.Read(f)
		if err != nil {
			return nil, err
		}
		p, err := parseProject(parts[1], text)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", f, err)
		}
		out = append(out, p)
	}
	return out, nil
}

func parseProject(dir, text string) (*Project, error) {
	val, err := decodeYAML(text)
	if err != nil {
		return nil, fmt.Errorf("not valid YAML (%s)", firstLine(err.Error()))
	}
	m, _ := val.(map[string]any)
	p := &Project{Name: dir, Dir: projectsDir + "/" + dir, Incoming: "open"}
	if n := get(m, "project"); n != nil {
		p.Name = pyStr(n)
	}
	p.Title = p.Name
	if t := get(m, "title"); t != nil {
		p.Title = pyStr(t)
	}
	for _, r := range asList(get(m, "owns")) {
		p.Owns = append(p.Owns, pyStr(r))
	}
	for _, d := range asList(get(m, "depends_on")) {
		switch x := d.(type) {
		case map[string]any:
			dep := Dependency{Repo: pyStr(get(x, "repo"))}
			for _, path := range asList(get(x, "paths")) {
				dep.Paths = append(dep.Paths, pyStr(path))
			}
			p.DependsOn = append(p.DependsOn, dep)
		default:
			p.DependsOn = append(p.DependsOn, Dependency{Repo: pyStr(x)})
		}
	}
	if i := get(m, "incoming"); i != nil {
		p.Incoming = pyStr(i)
	}
	return p, nil
}

// legacyProjectName is the project a code repository's docs/planning/
// belongs to: `planning.project` in .ordvik.yml, else the checkout's name.
func legacyProjectName(root string) string {
	if b, err := os.ReadFile(filepath.Join(root, ordvikConfig)); err == nil {
		if val, err := decodeYAML(string(b)); err == nil {
			if m, ok := val.(map[string]any); ok {
				if pl, ok := m["planning"].(map[string]any); ok {
					if n := get(pl, "project"); n != nil {
						return pyStr(n)
					}
				}
			}
		}
	}
	return filepath.Base(root)
}

// Project is the project named name, or nil.
func (v *View) Project(name string) *Project {
	for _, p := range v.Projects {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// ProjectOf is the project whose directory holds path, or nil.
func (v *View) ProjectOf(p string) *Project {
	for _, pr := range v.Projects {
		if strings.HasPrefix(p, pr.Dir+"/") {
			return pr
		}
	}
	return nil
}

// Qualify makes a bare id into <project>/<id>; a qualified one is kept.
func Qualify(project, id string) string {
	if strings.Contains(id, "/") {
		return id
	}
	return project + "/" + id
}

// Unqualify drops the project when it is home.
func Unqualify(home, id string) string {
	if strings.HasPrefix(id, home+"/") {
		return strings.TrimPrefix(id, home+"/")
	}
	return id
}

func asList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func joinPath(parts ...string) string { return path.Join(parts...) }

// CheckProjects: every repository has at most one owning project, and a
// project depends only on repositories some project owns.
func CheckProjects(projects []*Project) []string {
	var errors []string
	owner := map[string]string{}
	for _, p := range projects {
		if !projectName.MatchString(p.Name) {
			errors = append(errors, fmt.Sprintf("%s: project %q must be lowercase letters, digits and dashes", p.Dir, p.Name))
		}
		if p.Incoming != "open" && p.Incoming != "review" {
			errors = append(errors, fmt.Sprintf("%s: incoming %q is not open or review", p.Dir, p.Incoming))
		}
		for _, r := range p.Owns {
			if o, ok := owner[r]; ok {
				errors = append(errors, fmt.Sprintf("%s is owned by both %s and %s", r, o, p.Name))
				continue
			}
			owner[r] = p.Name
		}
	}
	for _, p := range projects {
		for _, d := range p.DependsOn {
			if _, ok := owner[d.Repo]; !ok {
				errors = append(errors, fmt.Sprintf("%s depends on %s, which no project owns", p.Name, d.Repo))
			}
		}
	}
	return errors
}
