#!/usr/bin/env python3
"""Tests for scripts/asks, run against any command that claims to be it.

The tests of the script, with the command taken from
ASKS_CMD: ASKS_CMD="ordvik asks" python3 tests/contract/test_asks.py
(go test runs it).

Each test builds a throwaway repository (the same way test_plan.py does)
with dispatch docs and RFCs committed on master, plans and ask files in the
working tree, then drives the real `asks` script with `--ref master`.
"""

import json
import os
import subprocess
import sys
import tempfile
import textwrap
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
from test_plan import Repo, dispatch  # noqa: E402

ASKS_CMD = os.environ["ASKS_CMD"].split()
TODAY = "2026-10-23"


def gated(thread, gates, *, tasks=3, done=(), status="active", plan=None):
    extra = "human_gates:\n" + "".join(f'  - "{g}"\n' for g in gates)
    return dispatch(thread, tasks=tasks, done=done, status=status, plan=plan, executor="mixed",
                    extra=extra.rstrip("\n"))


def ask_file(**fields):
    lines = ["---"] + [f"{k}: {json.dumps(v) if isinstance(v, (list, str)) else v}" for k, v in fields.items()]
    return "\n".join(lines + ["---", "", "Context.", ""])


class AsksTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Repo(self.tmp.name)

    def tearDown(self):
        self.tmp.cleanup()

    def run_asks(self, *args):
        r = subprocess.run(ASKS_CMD + [*args, "--ref", "master", "--today", TODAY],
                           cwd=self.repo.path, env=self.repo.env, capture_output=True, text=True)
        return r.returncode, r.stdout + r.stderr

    def check(self):
        r = subprocess.run(ASKS_CMD + ["check", "--ref", "master"],
                           cwd=self.repo.path, env=self.repo.env, capture_output=True, text=True)
        return r.returncode, r.stdout + r.stderr

    def asks(self):
        code, out = self.run_asks("status", "--json")
        self.assertEqual(code, 0, out)
        return {a["id"]: a for a in json.loads(out)["asks"]}

    def commit(self, files):
        for rel, text in files.items():
            self.repo.write(rel, text)
        self.repo.git("add", "-A")
        self.repo.git("commit", "-q", "-m", "files")

    def test_open_gates_are_asks_and_done_gates_are_not(self):
        self.repo.commit_docs({"300": gated("300", ["Task 2: Deploy to production", "Task 3: Review and accept"],
                                            done=(1, 2))})
        asks = self.asks()
        self.assertEqual(set(asks), {"dispatch:300:3"})
        self.assertEqual(asks["dispatch:300:3"]["kind"], "decide")
        self.assertEqual(asks["dispatch:300:3"]["unmarked"], [])

    def test_kind_and_categories_come_from_the_wording(self):
        self.repo.commit_docs({"301": gated("301", [
            "Task 1: Tailnet grant tag:a → tag:b:4222",
            "Task 2: The operator checks both apps on an iPhone",
            "Task 3: Confirm decisions D1–D3",
            "Task 4: First production converge of queue-01",
            "Task 5: Verify the dashboard and /health"], tasks=5)})
        asks = self.asks()
        self.assertEqual(asks["dispatch:301:1"]["kind"], "do")
        self.assertIn("tailnet-policy", asks["dispatch:301:1"]["categories"])
        self.assertEqual(asks["dispatch:301:2"]["kind"], "do")  # needs the operator's device
        self.assertIn("device-check", asks["dispatch:301:2"]["categories"])
        self.assertEqual(asks["dispatch:301:3"]["kind"], "decide")
        self.assertEqual(asks["dispatch:301:4"]["kind"], "approve")
        self.assertEqual(asks["dispatch:301:5"]["kind"], "review")

    def test_approve_asks_are_not_automation_candidates(self):
        self.repo.commit_docs({str(n): gated(str(n), ["Task 1: Production rollout with fleet upgrade"])
                               for n in (340, 341, 342)})
        code, out = self.run_asks("status")
        self.assertIn("Approve (3)", out)
        self.assertNotIn("Candidates to automate", out)

    def test_earlier_unmarked_tasks_are_flagged(self):
        self.repo.commit_docs({"302": gated("302", ["Task 3: Production rollout"], done=(1,))})
        a = self.asks()["dispatch:302:3"]
        self.assertEqual(a["unmarked"], ["2"])
        code, out = self.run_asks("status")
        self.assertIn("*302 Task 3", out)

    def test_closed_and_archived_dispatches_are_skipped(self):
        self.repo.commit_docs({
            "303": gated("303", ["Task 1: Deploy"], status="done"),
            "304": gated("304", ["Task 1: Deploy"], status="superseded"),
            "305": gated("305", ["Task 1: Deploy"]),
        }, archived=("305",))
        self.assertEqual(self.asks(), {})

    def test_lettered_gate_tasks(self):
        doc = gated("306", ["Task G1: Create the VM"], tasks=1).replace(
            "## Task 1: Step 1", "## Task 1: Step 1 — DONE\n\n## Task G1: Create the VM")
        self.repo.commit_docs({"306": doc})
        self.assertIn("dispatch:306:G1", self.asks())
        self.repo.commit_docs({"306": doc.replace("## Task G1: Create the VM", "## Task G1: Create the VM — DONE")})
        self.assertEqual(self.asks(), {})

    def test_plan_gates_and_what_a_dispatch_gate_unblocks(self):
        self.repo.commit_docs({"310": gated("310", ["Task 2: First production converge"], plan="demo"),
                               "311": dispatch("311", plan="demo")})
        self.repo.write("docs/planning/plans/demo.md", textwrap.dedent("""\
            ---
            plan: demo
            status: active
            threads:
              - {id: "310"}
              - {id: "311", after: ["310:2"]}
              - {id: "312", title: "Later", after: ["gate:go"]}
              - {id: "313", title: "Done gate", after: ["gate:old"]}
            gates:
              - {id: go, ask: "Start phase 2?"}
              - {id: old, ask: "Old question?", passed: 2026-10-01}
            ---
            """))
        asks = self.asks()
        self.assertEqual(asks["dispatch:310:2"]["unblocks"], ["311"])
        self.assertEqual(asks["plan:demo:go"]["unblocks"], ["312"])
        self.assertEqual(asks["plan:demo:go"]["kind"], "decide")
        self.assertNotIn("plan:demo:old", asks)

    def test_considering_rfcs_are_asks_and_drafts_are_counted(self):
        self.commit({
            "docs/rfcs/0001-a.md": "# RFC 0001: A\n\n**Status**: considering\n",
            "docs/rfcs/0002-b.md": "# RFC 0002: B\n\n**Status**: draft\n",
            "docs/rfcs/0003-c.md": "# RFC 0003: C\n\n**Status**: accepted\n",
            "search/docs/rfcs/0004-d.md": "---\nstatus: considering\n---\n# RFC 0004: D\n",
        })
        code, out = self.run_asks("status", "--json")
        data = json.loads(out)
        ids = {a["id"]: a for a in data["asks"]}
        self.assertEqual(set(ids), {"rfc:docs/rfcs/0001-a.md", "rfc:search/docs/rfcs/0004-d.md"})
        self.assertEqual(ids["rfc:search/docs/rfcs/0004-d.md"]["label"], "search RFC 0004")
        self.assertEqual(data["rfc_drafts"]["count"], 1)

    def test_ask_files_open_deferred_and_answered(self):
        self.repo.write("docs/planning/asks/raw-samples.md", ask_file(
            ask="Export three CR3 files", kind="do", category="files", raised="2026-10-17",
            unblocks=["photo-import RAW tests"], **{"from": "project thread: photo-import"}))
        self.repo.write("docs/planning/asks/later.md", ask_file(
            ask="Pick a DNS provider", kind="decide", raised="2026-10-01", defer_until="2026-11-01"))
        self.repo.write("docs/planning/asks/archive/old.md", ask_file(
            ask="Old", kind="do", raised="2026-09-01", answered="2026-09-02", outcome="done", answer="Did it."))
        asks = self.asks()
        self.assertEqual(set(asks), {"ask:raw-samples", "ask:later"})
        self.assertEqual(asks["ask:raw-samples"]["state"], "ready")
        self.assertEqual(asks["ask:raw-samples"]["age_days"], 6)
        self.assertEqual(asks["ask:later"]["state"], "deferred")
        code, out = self.check()
        self.assertEqual(code, 0, out)

    def test_check_reports_bad_ask_files_and_gate_entries(self):
        self.repo.commit_docs({"320": gated("320", ["Task 9: Deploy", "Deploy somewhere"])})
        self.repo.write("docs/planning/asks/bad-kind.md", ask_file(ask="X", kind="maybe", raised="2026-10-01"))
        self.repo.write("docs/planning/asks/answered.md", ask_file(
            ask="Y", kind="do", raised="2026-10-01", answered="2026-10-02"))
        code, out = self.check()
        self.assertEqual(code, 1)
        self.assertIn("bad-kind.md: kind 'maybe'", out)
        self.assertIn("answered.md: an answered ask needs outcome", out)
        self.assertIn("answered.md: answered; git mv it", out)
        self.assertIn("names Task 9, which has no `## Task` heading", out)
        self.assertIn("names no task", out)

    def test_recurring_do_kinds_are_automation_candidates(self):
        self.repo.commit_docs({str(n): gated(str(n), ["Task 1: Restore drill in production"])
                               for n in (330, 331, 332)})
        code, out = self.run_asks("status")
        self.assertIn("Candidates to automate", out)
        self.assertRegex(out, r"restore-drill\s+3")
        code, out = self.run_asks("status", "--json")
        self.assertEqual(len(json.loads(out)["recurring"]["restore-drill"]), 3)


if __name__ == "__main__":
    unittest.main()
