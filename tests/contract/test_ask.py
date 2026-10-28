#!/usr/bin/env python3
"""Tests for `ordvik planning ask`, raising and answering asks through git.

The command is taken from ORDVIK_CMD:
ORDVIK_CMD=ordvik python3 tests/contract/test_ask.py (go test runs it).

Each test builds a planning repository, pushes it to a bare remote and
works in a clone of it, the way a cloud session does: every ask and answer
is a commit pushed to the remote's main, and the clone reads them back
after a fetch.
"""

import json
import os
import re
import shutil
import subprocess
import tempfile
import textwrap
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
ORDVIK_CMD = os.environ["ORDVIK_CMD"].split()
SKILL = os.path.join(HERE, "..", "..", "plugins", "ordvik", "skills", "ask", "SKILL.md")

ENV_BASE = {
    "GIT_AUTHOR_NAME": "t",
    "GIT_AUTHOR_EMAIL": "t@example.invalid",
    "GIT_COMMITTER_NAME": "t",
    "GIT_COMMITTER_EMAIL": "t@example.invalid",
    "GIT_CONFIG_GLOBAL": "/dev/null",
    "GIT_CONFIG_NOSYSTEM": "1",
}

DISPATCH = textwrap.dedent("""\
    ---
    id: "20261028-parser"
    plan: rollout
    branch: feature/parser
    executor: mixed
    status: active
    source: test
    human_gates:
      - "Task 2: Deploy to production"
    ---

    # Parser

    ## Task 1: Build

    ## Task 2: Deploy

    ## Task 3: Clean up
    """)

PLAN = textwrap.dedent("""\
    ---
    plan: rollout
    status: active
    dispatches:
      - {id: "20261028-parser"}
      - {id: "20261028-next", title: "Next", after: ["gate:go"]}
    gates:
      - {id: go, ask: "Start the next phase?"}
    ---

    # Rollout
    """)

OLD_ASK = textwrap.dedent("""\
    ---
    ask: Export three sample files
    kind: do
    category: files
    from: "a thread"
    raised: 2026-10-20
    ---

    Context.
    """)


def git(cwd, *args, env):
    return subprocess.run(["git", *args], cwd=cwd, env=env, capture_output=True, text=True, check=True).stdout


class AskTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        base = self.tmp.name
        self.env = dict(os.environ)
        self.env.update(ENV_BASE)
        src = os.path.join(base, "src")
        files = {
            "planning.yml": "format: 1\nask_url: \"https://inbox.example/asks/{id}\"\n",
            "projects/demo/project.yml": "project: demo\ntitle: Demo\nowns: [acme/app]\n",
            "projects/demo/dispatch/20261028-parser.md": DISPATCH,
            "projects/demo/plans/rollout.md": PLAN,
            "projects/demo/asks/export-samples.md": OLD_ASK,
        }
        for rel, text in files.items():
            p = os.path.join(src, rel)
            os.makedirs(os.path.dirname(p), exist_ok=True)
            with open(p, "w") as f:
                f.write(text)
        git(src, "init", "-q", "-b", "main", env=self.env)
        git(src, "add", "-A", env=self.env)
        git(src, "commit", "-q", "-m", "base", env=self.env)
        self.remote = os.path.join(base, "remote.git")
        git(base, "clone", "-q", "--bare", src, self.remote, env=self.env)
        self.path = self.clone("planning")

    def tearDown(self):
        self.tmp.cleanup()

    def clone(self, name):
        path = os.path.join(self.tmp.name, name)
        git(self.tmp.name, "clone", "-q", self.remote, path, env=self.env)
        return path

    def run_cmd(self, *args, cwd=None):
        r = subprocess.run(ORDVIK_CMD + ["planning", *args], cwd=cwd or self.path, env=self.env,
                           capture_output=True, text=True)
        return r.returncode, r.stdout + r.stderr

    def ok(self, *args, cwd=None):
        code, out = self.run_cmd(*args, cwd=cwd)
        self.assertEqual(code, 0, out)
        return out

    def new(self, *extra, cwd=None):
        out = self.ok("ask", "new", "--question", "Ship the parser behind a flag?",
                      "--option", "Behind a flag=the old parser stays default for a week",
                      "--option", "Replace=the old parser goes now",
                      "--recommended", "0", "--context", "Two bugs this week.",
                      "--from-pr", "acme/app#12", "--from-thread", "Parser rewrite",
                      "--today", "2026-10-28", *extra, cwd=cwd)
        return re.search(r"Asked (\S+) and pushed", out).group(1)

    def remote_show(self, rel):
        return git(self.remote, "show", "main:" + rel, env=self.env)

    def asks(self):
        out = self.ok("asks", "--json", "--no-prs", "--today", "2026-10-30")
        return {a["id"]: a for a in json.loads(out[out.index("{"):])["asks"]}

    def test_new_is_pushed_and_listed_with_its_shape(self):
        out = self.ok("ask", "new", "--project", "demo", "--question", "Ship the parser behind a flag?",
                      "--option", "Behind a flag=the old parser stays default for a week",
                      "--option", "Replace=the old parser goes now", "--recommended", "0",
                      "--today", "2026-10-28")
        self.assertIn("Asked ask:demo/ship-the-parser-behind-a-flag and pushed.", out)
        self.assertIn("https://inbox.example/asks/ask:demo/ship-the-parser-behind-a-flag", out)
        self.assertIn("ask: \"Ship the parser behind a flag?\"",
                      self.remote_show("projects/demo/asks/ship-the-parser-behind-a-flag.md"))
        a = self.asks()["ask:demo/ship-the-parser-behind-a-flag"]
        self.assertEqual(a["kind"], "decide")
        self.assertEqual(a["options"][1], {"label": "Replace", "then": "the old parser goes now"})
        self.assertEqual(a["recommended"], 0)
        self.assertEqual(a["raised"], "2026-10-28")
        self.assertIn("ask:demo/export-samples", self.asks())  # the old-format file is still an ask
        self.assertEqual(self.run_cmd("asks", "check")[0], 0)

    def test_new_slugs_are_unique(self):
        first = self.new("--project", "demo")
        second = self.new("--project", "demo")
        self.assertEqual(first, "ask:demo/ship-the-parser-behind-a-flag")
        self.assertEqual(second, "ask:demo/ship-the-parser-behind-a-flag-2")

    def test_a_bad_ask_is_refused_before_anything_is_written(self):
        for extra, want in [
            (["--option", "Only=one", "--recommended", "0"], "2 to 4 options"),
            (["--option", "A=a", "--option", "B=b", "--recommended", "2"], "0 to 1"),
            (["--option", "A=a", "--option", "B", "--recommended", "0"], "label=then"),
            (["--option", "A=a", "--option", "B=b"], "--recommended"),
        ]:
            code, out = self.run_cmd("ask", "new", "--project", "demo", "--question", "Q?", *extra)
            self.assertEqual(code, 2, out)
            self.assertIn(want, out)
        self.assertEqual(git(self.remote, "rev-list", "--count", "main", env=self.env).strip(), "1")

    def test_answer_archives_and_show_returns_it(self):
        ident = self.new("--project", "demo")
        code, out = self.run_cmd("ask", "answer", ident, "other")
        self.assertEqual(code, 1)
        self.assertIn("needs --text", out)
        self.ok("ask", "answer", ident, "other", "--text", "a flag, for two days", "--by", "Operator",
                "--today", "2026-10-29")
        archived = self.remote_show("projects/demo/asks/archive/ship-the-parser-behind-a-flag.md")
        self.assertIn("answered: 2026-10-29\nanswer: other\n", archived)
        self.assertIn("outcome: decided", archived)
        self.assertNotIn(ident, self.asks())
        shown = json.loads(self.ok("ask", "show", ident, "--json"))
        self.assertEqual((shown["answer"], shown["answer_text"], shown["answered_by"]),
                         ("other", "a flag, for two days", "Operator"))
        self.assertIn("answer (2026-10-29 by Operator): other: a flag, for two days",
                      self.ok("ask", "show", "ship-the-parser-behind-a-flag"))
        code, out = self.run_cmd("ask", "answer", ident, "0")
        self.assertEqual(code, 1)
        self.assertIn("already answered", out)
        self.assertEqual(self.run_cmd("asks", "check")[0], 0)

    def test_wait_returns_on_answer_and_times_out_otherwise(self):
        ident = self.new("--project", "demo")
        code, out = self.run_cmd("ask", "wait", ident, "--timeout", "1s", "--interval", "200ms")
        self.assertEqual(code, 3, out)
        self.assertIn("decide, open", out)
        # The operator answers from another clone; the waiting one fetches it.
        other = self.clone("operator")
        self.ok("ask", "answer", ident, "1", "--by", "Operator", cwd=other)
        code, out = self.run_cmd("ask", "wait", ident, "--timeout", "5s", "--interval", "200ms")
        self.assertEqual(code, 0, out)
        self.assertIn("1. Replace", out)

    def test_gates_take_the_shape_and_old_gates_stay_valid(self):
        self.assertEqual(self.run_cmd("asks", "check")[0], 0)
        self.assertEqual(self.new("--gate", "demo/20261028-parser:2"), "dispatch:demo/20261028-parser:2")
        self.assertEqual(self.new("--gate", "demo/20261028-parser:3"), "dispatch:demo/20261028-parser:3")
        self.assertEqual(self.new("--gate", "plan:demo/rollout:go"), "plan:demo/rollout:go")
        doc = self.remote_show("projects/demo/dispatch/20261028-parser.md")
        self.assertIn('human_gates:\n  - ask: "Task 2: Ship the parser behind a flag?"\n'
                      '    from: {thread: "Parser rewrite", pr: "acme/app#12"}\n    options:\n', doc)
        self.assertIn('  - ask: "Task 3: Ship the parser behind a flag?"\n', doc)
        self.assertIn("status: active\nsource: test\n", doc)  # other lines are kept as written
        plan = self.remote_show("projects/demo/plans/rollout.md")
        self.assertIn('gates:\n  - id: go\n    ask: "Ship the parser behind a flag?"\n', plan)
        asks = self.asks()
        for ident in ("dispatch:demo/20261028-parser:2", "plan:demo/rollout:go"):
            self.assertEqual(asks[ident]["kind"], "decide")
            self.assertEqual(asks[ident]["recommended"], 0)
            self.assertEqual(asks[ident]["from"], {"thread": "Parser rewrite", "pr": "acme/app#12"})
        self.assertEqual(self.run_cmd("asks", "check")[0], 0)
        self.ok("ask", "answer", "demo/20261028-parser:2", "0")
        self.ok("ask", "answer", "plan:demo/rollout:go", "1", "--text", "after the holidays")
        asks = self.asks()
        self.assertNotIn("dispatch:demo/20261028-parser:2", asks)
        self.assertNotIn("plan:demo/rollout:go", asks)
        self.assertIn("    answered: ", self.remote_show("projects/demo/plans/rollout.md"))
        self.assertEqual(self.run_cmd("asks", "check")[0], 0)
        self.assertEqual(self.run_cmd("check")[0], 0)

    def test_a_push_race_starts_again_on_the_new_tip(self):
        # Another writer pushes between this clone's fetch and its push: the
        # pre-push hook pushes a commit from a second clone, once.
        other = self.clone("other")
        marker = os.path.join(self.tmp.name, "raced")
        hook = os.path.join(self.path, ".git", "hooks", "pre-push")
        with open(hook, "w") as f:
            f.write(textwrap.dedent(f"""\
                #!/bin/sh
                [ -e {marker} ] && exit 0
                touch {marker}
                cd {other} && echo x > other.txt && git add other.txt && git commit -q -m other && git push -q origin HEAD:main
                """))
        os.chmod(hook, 0o755)
        ident = self.new("--project", "demo")
        self.assertTrue(os.path.exists(marker))
        log = git(self.remote, "log", "--format=%s", "main", env=self.env).split("\n")
        self.assertEqual(log[:2], ["demo: ask ship-the-parser-behind-a-flag", "other"])
        self.assertIn(ident, self.asks())

    def test_the_skill_example_runs(self):
        with open(SKILL) as f:
            text = f.read()
        block = next(b for b in re.findall(r"```bash\n(.*?)```", text, re.S) if "ask new" in b)
        bindir = os.path.join(self.tmp.name, "bin")
        os.mkdir(bindir)
        binary = shutil.which(ORDVIK_CMD[0]) or ORDVIK_CMD[0]
        os.symlink(os.path.abspath(binary), os.path.join(bindir, "ordvik"))
        env = dict(self.env, PATH=bindir + os.pathsep + self.env["PATH"])
        r = subprocess.run(["sh", "-ec", block], cwd=self.path, env=env, capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("Asked ask:demo/", r.stdout)
        self.assertEqual(self.run_cmd("asks", "check")[0], 0)


if __name__ == "__main__":
    unittest.main()
