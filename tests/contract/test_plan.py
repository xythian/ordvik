#!/usr/bin/env python3
"""Tests for scripts/plan, run against any command that claims to be it.

The tests of the script, with the command taken from
PLAN_CMD, so `ordvik plan` is held to the script's behaviour:
PLAN_CMD="ordvik plan" python3 tests/contract/test_plan.py (go test runs it).

Each test builds a throwaway repository with dispatch docs committed on
master and a plan in the working tree, then drives the real `plan` script
as a subprocess with `--ref master`, so what is tested is what the
conductor of a plan reads.
"""

import json
import os
import subprocess
import sys
import tempfile
import textwrap
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
PLAN_CMD = os.environ["PLAN_CMD"].split()

ENV_BASE = {
    "GIT_AUTHOR_NAME": "t",
    "GIT_AUTHOR_EMAIL": "t@example.invalid",
    "GIT_COMMITTER_NAME": "t",
    "GIT_COMMITTER_EMAIL": "t@example.invalid",
    "GIT_CONFIG_GLOBAL": "/dev/null",
    "GIT_CONFIG_NOSYSTEM": "1",
}


def dispatch(thread, *, status="active", plan="demo", branch=None, executor="agent",
             tasks=2, done=(), extra=""):
    lines = ["---", f'thread: "{thread}"', f"branch: {branch or 'feature/t' + thread}",
             f"executor: {executor}", f"status: {status}", "source: test"]
    if plan:
        lines.append(f"plan: {plan}")
    if extra:
        lines.append(extra)
    lines += ["---", "", f"# Thread {thread}: Title {thread}", ""]
    for n in range(1, tasks + 1):
        lines += [f"## Task {n}: Step {n}" + (" — DONE" if n in done else ""), ""]
    return "\n".join(lines) + "\n"


PLAN_DOC = textwrap.dedent("""\
    ---
    plan: demo
    title: Demo plan
    status: active
    conductor: test thread
    threads:
      - {id: "200"}
      - {id: "201"}
      - {id: "202", after: ["200:1"]}
      - {id: "203", after: ["200", "201"]}
      - {id: "204", title: "Later work", after: ["203", "gate:go"]}
    gates:
      - {id: go, ask: "Go ahead with 204?"}
    ---

    # Demo plan
    """)


class Repo:
    def __init__(self, base):
        self.path = os.path.join(base, "repo")
        self.env = dict(os.environ)
        self.env.update(ENV_BASE)
        os.mkdir(self.path)
        self.git("init", "-q", "-b", "master")
        self.write("README.md", "x\n")
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "base")

    def git(self, *args):
        return subprocess.run(["git", *args], cwd=self.path, env=self.env,
                              capture_output=True, text=True, check=True).stdout

    def write(self, rel, text):
        p = os.path.join(self.path, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w") as f:
            f.write(text)

    def commit_docs(self, docs, archived=()):
        """Commit dispatch docs {thread: text}; threads in archived go
        under dispatch/archive/, as a closed dispatch does."""
        for thread, text in docs.items():
            sub = "archive/" if thread in archived else ""
            for d in ("", "archive/"):
                stale = os.path.join(self.path, f"docs/planning/dispatch/{d}{thread}-t.md")
                if os.path.exists(stale):
                    os.remove(stale)
            self.write(f"docs/planning/dispatch/{sub}{thread}-t.md", text)
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "docs")

    def plan(self, *args):
        r = subprocess.run(PLAN_CMD + [*args, "--ref", "master"], cwd=self.path,
                           env=self.env, capture_output=True, text=True)
        return r.returncode, r.stdout + r.stderr

    def states(self, plan="demo"):
        code, out = self.plan("status", plan, "--json")
        assert code == 0, out
        return {t["id"]: t["state"] for t in json.loads(out)["threads"]}


class PlanTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Repo(self.tmp.name)
        self.repo.write("docs/planning/plans/demo.md", PLAN_DOC)

    def tearDown(self):
        self.tmp.cleanup()

    def base_docs(self, **over):
        docs = {t: dispatch(t) for t in ("200", "201", "202", "203")}
        docs.update(over)
        return docs

    def test_first_stage_is_ready_and_the_rest_wait(self):
        self.repo.commit_docs(self.base_docs())
        self.assertEqual(self.repo.states(), {
            "200": "ready", "201": "ready", "202": "blocked", "203": "blocked", "204": "planned"})
        code, out = self.repo.plan("ready", "demo")
        self.assertEqual(out.split(), ["docs/planning/dispatch/200-t.md", "docs/planning/dispatch/201-t.md"])

    def test_task_done_on_master_unblocks_a_task_dependency(self):
        self.repo.commit_docs(self.base_docs(**{"200": dispatch("200", done=(1,))}))
        states = self.repo.states()
        self.assertEqual(states["200"], "started")  # a task is done, so it was launched
        self.assertEqual(states["202"], "ready")
        self.assertEqual(states["203"], "blocked")

    def test_archived_done_threads_satisfy_dependencies(self):
        docs = self.base_docs(**{"200": dispatch("200", status="done", done=(1, 2)),
                                 "201": dispatch("201", status="done", done=(1, 2))})
        self.repo.commit_docs(docs, archived=("200", "201"))
        states = self.repo.states()
        self.assertEqual((states["200"], states["201"]), ("done", "done"))
        self.assertEqual(states["203"], "ready")
        self.assertEqual(states["204"], "planned")  # still waits on 203 and the gate

    def test_sketch_needs_a_doc_once_its_conditions_hold(self):
        docs = self.base_docs(**{t: dispatch(t, status="done") for t in ("200", "201", "203")})
        self.repo.commit_docs(docs)
        self.assertEqual(self.repo.states()["204"], "planned")
        self.repo.write("docs/planning/plans/demo.md", PLAN_DOC.replace(
            '{id: go, ask: "Go ahead with 204?"}', '{id: go, ask: "Go ahead with 204?", passed: 2030-01-02}'))
        self.assertEqual(self.repo.states()["204"], "write-doc")

    def test_stages_come_from_what_threads_wait_on(self):
        self.repo.commit_docs(self.base_docs())
        code, out = self.repo.plan("status", "demo", "--json")
        stages = {t["id"]: t["stage"] for t in json.loads(out)["threads"]}
        # 202 waits on part of 200, 203 on all of 200 and 201; gates add no stage.
        self.assertEqual(stages, {"200": 1, "201": 1, "202": 2, "203": 2, "204": 3})

    def test_launched_sketch_is_started_not_write_doc(self):
        docs = self.base_docs(**{t: dispatch(t, status="done") for t in ("200", "201", "203")})
        self.repo.commit_docs(docs)
        self.repo.write("docs/planning/plans/demo.md", PLAN_DOC
                        .replace('{id: go, ask: "Go ahead with 204?"}', '{id: go, ask: "Go ahead with 204?", passed: 2030-01-02}')
                        .replace('title: "Later work",', 'title: "Later work", launched: "2030-01-03 from a PR",'))
        self.assertEqual(self.repo.states()["204"], "started")

    def test_pending_gate_is_reported_with_what_it_unblocks(self):
        self.repo.commit_docs(self.base_docs())
        code, out = self.repo.plan("status", "demo")
        self.assertEqual(code, 0, out)
        self.assertIn("Gates to ask the operator:", out)
        self.assertIn("go: Go ahead with 204? (unblocks 204)", out)
        self.assertIn("waits on: 200 Task 1", out)
        self.assertIn("Task 1 unblocks: 202", out)

    def test_remote_branch_or_launched_marks_started(self):
        self.repo.commit_docs(self.base_docs())
        self.repo.git("update-ref", "refs/remotes/origin/feature/t200", "HEAD")
        self.repo.write("docs/planning/plans/demo.md", PLAN_DOC.replace(
            '{id: "201"}', '{id: "201", launched: "2030-01-01 worker-01"}'))
        states = self.repo.states()
        self.assertEqual((states["200"], states["201"]), ("started", "started"))
        code, out = self.repo.plan("ready", "demo")
        self.assertEqual(out.strip(), "")

    def test_human_and_dropped_threads_are_never_launched(self):
        self.repo.commit_docs(self.base_docs(**{"201": dispatch("201", executor="human")}))
        self.repo.write("docs/planning/plans/demo.md", PLAN_DOC.replace(
            '{id: "200"}', '{id: "200", dropped: "not needed"}'))
        states = self.repo.states()
        self.assertEqual((states["200"], states["201"]), ("dropped", "human"))

    def test_check_passes_a_consistent_plan(self):
        self.repo.commit_docs(self.base_docs())
        code, out = self.repo.plan("check", "demo")
        self.assertEqual(code, 0, out)
        self.assertIn("no errors", out)

    def test_check_finds_bad_references_cycles_and_strays(self):
        self.repo.commit_docs(self.base_docs(**{"299": dispatch("299")}))
        self.repo.write("docs/planning/plans/demo.md", PLAN_DOC
                        .replace('after: ["200:1"]', 'after: ["200:9", "gate:nope", "999"]')
                        .replace('{id: "200"}', '{id: "200", after: ["203"]}'))
        code, out = self.repo.plan("check", "demo")
        self.assertEqual(code, 1, out)
        self.assertIn("has no Task 9", out)
        self.assertIn("names no gate", out)
        self.assertIn("after 999 names a thread not in the plan", out)
        self.assertIn("cycle: 200 -> 203 -> 200", out)
        self.assertIn("299-t.md says plan demo but the plan does not list thread 299", out)

    def test_check_reports_frontmatter_that_is_not_yaml(self):
        bad = dispatch("201", extra='notes: "a: b"; c: d').replace('source: test', 'source: x "y: z"; w')
        self.repo.commit_docs(self.base_docs(**{"201": bad}))
        code, out = self.repo.plan("check", "demo")
        self.assertEqual(code, 1, out)
        self.assertIn("201-t.md: frontmatter is not valid YAML", out)
        # Status still places the thread from its loose frontmatter.
        self.assertEqual(self.repo.states()["201"], "ready")

    def test_check_warns_when_a_doc_disagrees_with_the_plan(self):
        self.repo.commit_docs(self.base_docs(**{
            "202": dispatch("202", extra='depends_on: ["201"]'),
            "203": dispatch("203", plan="other"),
            "201": dispatch("201", extra='wave: "004"')}))
        self.repo.write("docs/planning/plans/demo.md", PLAN_DOC.replace(
            '{id: "200"}', '{id: "200", wave: "001"}'))
        code, out = self.repo.plan("check", "demo")
        self.assertEqual(code, 0, out)
        self.assertIn("202-t.md: depends_on ['201'] differs from the plan's after ['200:1']", out)
        self.assertIn("203-t.md: frontmatter plan is 'other', expected 'demo'", out)
        self.assertIn("201-t.md: has wave 004; plans have no waves", out)
        self.assertIn("thread 200: plans have no waves", out)

    def test_named_threads_work_like_numbered_ones(self):
        named = "20261023-ops-catalog"
        self.repo.commit_docs(self.base_docs(**{named: dispatch(named, done=(1,))}))
        self.repo.write("docs/planning/plans/demo.md", PLAN_DOC.replace(
            '{id: "201"}', f'{{id: "201"}}\n  - {{id: "{named}"}}').replace(
            'after: ["200", "201"]', f'after: ["200", "201", "{named}:1"]'))
        code, out = self.repo.plan("check", "demo")
        self.assertEqual(code, 0, out)
        self.assertEqual(self.repo.states()[named], "started")

    def test_check_without_a_plan_finds_shared_thread_ids(self):
        self.repo.commit_docs({"300": dispatch("300", plan=None)})
        self.repo.write("docs/planning/dispatch/300-other.md", dispatch("300", plan=None))
        self.repo.commit_docs({"301": dispatch("301", plan=None)}, archived=("301",))
        self.repo.write("docs/planning/dispatch/301-new.md", dispatch("301", plan=None))
        self.repo.git("add", "-A")
        self.repo.git("commit", "-q", "-m", "clash")
        code, out = self.repo.plan("check")
        self.assertEqual(code, 1, out)
        self.assertIn("thread 300 is claimed by 2 dispatch docs", out)
        self.assertIn("warning: thread 301: docs/planning/dispatch/301-new.md shares its id", out)


if __name__ == "__main__":
    unittest.main()
