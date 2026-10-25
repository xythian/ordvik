package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The legacy layout is tested by the scripts' own tests in
// tests/contract (TestContract); these cover the planning repository.

type repo struct {
	t    *testing.T
	path string
}

func newRepo(t *testing.T, base, name, branch string) *repo {
	t.Helper()
	r := &repo{t: t, path: filepath.Join(base, name)}
	if err := os.MkdirAll(r.path, 0o755); err != nil {
		t.Fatal(err)
	}
	r.git("init", "-q", "-b", branch)
	return r
}

func (r *repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.path
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func (r *repo) write(files map[string]string) {
	r.t.Helper()
	for rel, text := range files {
		p := filepath.Join(r.path, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			r.t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *repo) commit(files map[string]string) {
	r.t.Helper()
	r.write(files)
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "files")
}

func (r *repo) run(args ...string) (int, string) {
	r.t.Helper()
	old := dir
	dir = r.path
	defer func() { dir = old }()
	var out bytes.Buffer
	code := run(args, &out, &out)
	return code, out.String()
}

func dispatchDoc(id string, extra string, tasks int, done ...int) string {
	var b strings.Builder
	b.WriteString("---\nid: \"" + id + "\"\nbranch: feature/" + id + "\nexecutor: agent\nsource: test\n")
	if extra != "" {
		b.WriteString(extra + "\n")
	}
	b.WriteString("---\n\n# " + id + ": Title\n\n")
	for n := 1; n <= tasks; n++ {
		heading := "## Task " + string(rune('0'+n)) + ": Step"
		for _, d := range done {
			if d == n {
				heading += " — DONE"
			}
		}
		b.WriteString(heading + "\n\n")
	}
	return b.String()
}

const (
	platformProject = "project: platform\ntitle: Platform\nowns: [acme/platform]\n"
	gameProject     = `project: game
title: Game
owns: [acme/game-server]
depends_on:
  - repo: acme/platform
    paths: [game/]
`
	gamePlan = `---
plan: moo
status: active
dispatches:
  - {id: "20261025-moo-a"}
  - {id: "20261025-moo-b", after: ["20261025-moo-a", "platform/20261025-hosting:1"]}
---
`
)

func planningRepo(t *testing.T) *repo {
	r := newRepo(t, t.TempDir(), "planning", "main")
	r.commit(map[string]string{
		"planning.yml":                                   "version: 1\n",
		"projects/platform/project.yml":                  platformProject,
		"projects/game/project.yml":                      gameProject,
		"projects/game/plans/moo.md":                     gamePlan,
		"projects/platform/dispatch/20261025-hosting.md": dispatchDoc("20261025-hosting", "thread: game-hosting", 2),
		"projects/game/dispatch/20261025-moo-a.md":       dispatchDoc("20261025-moo-a", "plan: moo\nrepo: acme/game-server", 1, 1),
		"projects/game/dispatch/20261025-moo-b.md":       dispatchDoc("20261025-moo-b", "plan: moo\nrepo: acme/platform", 1),
	})
	return r
}

func states(t *testing.T, r *repo, plan string) map[string]string {
	t.Helper()
	code, out := r.run("planning", "status", plan, "--json", "--ref", "main")
	if code != 0 {
		t.Fatalf("status: %d\n%s", code, out)
	}
	var data struct {
		Project string
		Threads []struct{ ID, State, Repo, Project string }
	}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if data.Project != "game" {
		t.Errorf("project = %q", data.Project)
	}
	got := map[string]string{}
	for _, th := range data.Threads {
		got[th.ID] = th.State
	}
	return got
}

func TestCrossProjectWait(t *testing.T) {
	r := planningRepo(t)
	got := states(t, r, "game/moo")
	if got["20261025-moo-a"] != "started" || got["20261025-moo-b"] != "blocked" {
		t.Fatalf("states = %v", got)
	}
	// moo-a done and platform's Task 1 done: moo-b can start.
	r.commit(map[string]string{
		"projects/game/dispatch/20261025-moo-a.md":       dispatchDoc("20261025-moo-a", "plan: moo\nstatus: done", 1, 1),
		"projects/platform/dispatch/20261025-hosting.md": dispatchDoc("20261025-hosting", "thread: game-hosting", 2, 1),
	})
	got = states(t, r, "moo") // a bare slug finds the one plan of that name
	if got["20261025-moo-b"] != "ready" {
		t.Fatalf("states = %v", got)
	}
	code, out := r.run("planning", "ready", "game/moo", "--ref", "main")
	if code != 0 || strings.TrimSpace(out) != "projects/game/dispatch/20261025-moo-b.md" {
		t.Fatalf("ready: %d %q", code, out)
	}
	code, out = r.run("planning", "check", "game/moo")
	if code != 0 {
		t.Fatalf("check: %d\n%s", code, out)
	}
}

func TestCheckFindsBadCrossProjectRefsAndOwners(t *testing.T) {
	r := planningRepo(t)
	r.write(map[string]string{
		"projects/game/plans/moo.md": strings.Replace(gamePlan, "platform/20261025-hosting:1", "platform/nope:1", 1),
		"projects/other/project.yml": "project: other\nowns: [acme/platform]\ndepends_on: [acme/unowned]\n",
	})
	// check reads the working tree in the planning repository, so the
	// uncommitted edit is what it checks.
	code, out := r.run("planning", "check", "game/moo")
	if code != 1 || !strings.Contains(out, "after platform/nope:1 names no dispatch doc") {
		t.Fatalf("check plan: %d\n%s", code, out)
	}
	code, out = r.run("planning", "check")
	if code != 1 || !strings.Contains(out, "acme/platform is owned by both") ||
		!strings.Contains(out, "other depends on acme/unowned, which no project owns") {
		t.Fatalf("check: %d\n%s", code, out)
	}
}

func TestSameIDInTwoProjectsIsFine(t *testing.T) {
	r := planningRepo(t)
	r.commit(map[string]string{
		"projects/platform/dispatch/20261025-moo-a.md": dispatchDoc("20261025-moo-a", "", 1),
	})
	code, out := r.run("planning", "check")
	if code != 0 {
		t.Fatalf("check: %d\n%s", code, out)
	}
	r.commit(map[string]string{
		"projects/platform/dispatch/20261025-moo-a-copy.md": dispatchDoc("20261025-moo-a", "", 1),
	})
	code, out = r.run("planning", "check")
	if code != 1 || !strings.Contains(out, "thread platform/20261025-moo-a is claimed by 2 dispatch docs") {
		t.Fatalf("check: %d\n%s", code, out)
	}
}

func TestAsksAcrossProjectsAndRFCsFromCheckouts(t *testing.T) {
	r := planningRepo(t)
	r.commit(map[string]string{
		"projects/game/dispatch/20261025-moo-gate.md": dispatchDoc("20261025-moo-gate",
			"human_gates:\n  - \"Task 1: Approve the production rollout\"", 1),
		"projects/platform/asks/pick-dns.md": "---\nask: Pick a DNS provider\nkind: decide\nraised: 2026-10-20\n---\n",
	})
	code := newRepo(t, filepath.Dir(r.path), "platform", "master")
	code.commit(map[string]string{"docs/rfcs/0001-a.md": "# RFC 0001: A\n\n**Status**: considering\n"})
	code.git("update-ref", "refs/remotes/origin/master", "HEAD")
	t.Setenv("ORDVIK_REPOS", "")

	status, out := r.run("planning", "asks", "--json", "--ref", "main", "--today", "2026-10-25")
	if status != 0 {
		t.Fatalf("asks: %d\n%s", status, out)
	}
	var data struct {
		Asks []struct {
			ID, Project, Repo, Kind, Label string
			AgeDays                        *int `json:"age_days"`
		}
		Notes []string
	}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	ids := map[string]string{}
	for _, a := range data.Asks {
		ids[a.ID] = a.Project + " " + a.Repo + " " + a.Kind
	}
	want := map[string]string{
		"dispatch:game/20261025-moo-gate:1":     "game  approve",
		"ask:platform/pick-dns":                 "platform  decide",
		"rfc:acme/platform:docs/rfcs/0001-a.md": "platform acme/platform decide",
	}
	for id, w := range want {
		if ids[id] != w {
			t.Errorf("ask %s = %q, want %q (all: %v)", id, ids[id], w, ids)
		}
	}
	// game-server has no checkout beside the planning repository.
	if len(data.Notes) != 1 || !strings.Contains(data.Notes[0], "RFCs of acme/game-server not read") {
		t.Errorf("notes = %v", data.Notes)
	}
	status, out = r.run("planning", "asks", "--ref", "main", "--today", "2026-10-25", "--project", "platform")
	if status != 0 || strings.Contains(out, "moo-gate") || !strings.Contains(out, "platform/pick-dns") {
		t.Fatalf("asks --project: %d\n%s", status, out)
	}
}

func TestLegacyThreadFieldInPlanningRepo(t *testing.T) {
	// A doc not yet renamed (thread: instead of id:) is still found by it.
	r := planningRepo(t)
	r.commit(map[string]string{
		"projects/game/dispatch/20261025-moo-a.md": strings.Replace(
			dispatchDoc("20261025-moo-a", "plan: moo", 1, 1), "id: ", "thread: ", 1),
	})
	if got := states(t, r, "game/moo"); got["20261025-moo-a"] != "started" {
		t.Fatalf("states = %v", got)
	}
}

// TestContract runs scripts/plan's and scripts/asks' own tests against
// `ordvik plan` and `ordvik asks`.
func TestContract(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	if err := exec.Command("python3", "-c", "import yaml").Run(); err != nil {
		t.Skip("PyYAML is not installed")
	}
	bin := filepath.Join(t.TempDir(), "ordvik")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	for _, f := range []string{"test_plan.py", "test_asks.py"} {
		cmd := exec.Command("python3", f)
		cmd.Dir = "../../tests/contract"
		cmd.Env = append(os.Environ(), "PLAN_CMD="+bin+" plan", "ASKS_CMD="+bin+" asks")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("%s: %v\n%s", f, err, out)
		}
	}
}

func TestRepoField(t *testing.T) {
	r := planningRepo(t)
	r.commit(map[string]string{
		"projects/game/dispatch/20261025-moo-a.md": dispatchDoc("20261025-moo-a",
			"plan: moo\nrepo:\n  path: ../game-server\n  url: git@github.com:acme/game-server.git", 1),
	})
	code, out := r.run("planning", "status", "game/moo", "--json", "--ref", "main")
	if code != 0 {
		t.Fatalf("status: %d\n%s", code, out)
	}
	var data struct {
		Threads []struct{ ID, Repo, Branch string }
	}
	json.Unmarshal([]byte(out), &data)
	got := map[string]string{}
	for _, th := range data.Threads {
		got[th.ID] = th.Repo + " " + th.Branch
	}
	if got["20261025-moo-a"] != "acme/game-server feature/20261025-moo-a" ||
		got["20261025-moo-b"] != "acme/platform feature/20261025-moo-b" {
		t.Fatalf("repos = %v", got)
	}
}
