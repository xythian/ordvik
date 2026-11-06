#!/usr/bin/env python3
"""Tests for the plugin's scripts/rfc.

Run directly: python3 tests/contract/test_rfc.py (go test runs it).

Each test builds a throwaway repository with an RFC scope committed on
master, then drives the real `rfc` script as a subprocess, so what is
tested is what a thread and the reservation workflow run.
"""

import os
import subprocess
import sys
import tempfile
import textwrap
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
RFC = os.path.join(HERE, "..", "..", "plugins", "ordvik", "scripts", "rfc")

ENV_BASE = {
    "GIT_AUTHOR_NAME": "t",
    "GIT_AUTHOR_EMAIL": "t@example.invalid",
    "GIT_COMMITTER_NAME": "t",
    "GIT_COMMITTER_EMAIL": "t@example.invalid",
    "GIT_CONFIG_GLOBAL": "/dev/null",
    "GIT_CONFIG_NOSYSTEM": "1",
}

README = textwrap.dedent("""\
    # RFCs

    | RFC | Title | Status | Notes |
    |-----|-------|--------|-------|
    | 0001 | [One](0001-one.md) | accepted | |
    | 0002 | [Two](0002-two.md) | draft | |

    Next number: **0003**

    ### Number Collisions

    | Number | File A | File B |
    |--------|--------|--------|
    """)


class Repo:
    def __init__(self, tmp):
        self.root = tmp
        self.env = {**os.environ, **ENV_BASE}
        self.git("init", "-q", "-b", "master")
        self.write("docs/rfcs/README.md", README)
        self.write("docs/rfcs/0001-one.md", "# RFC 0001: One\n\n**Status**: accepted\n")
        self.write("docs/rfcs/0002-two.md", "# RFC 0002: Two\n\n**Status**: draft\n")
        self.write("src/main.go", "package main\n")
        self.commit("base")

    def git(self, *args):
        r = subprocess.run(["git", "-C", self.root, *args], capture_output=True, text=True, env=self.env)
        if r.returncode != 0:
            raise AssertionError(f"git {args}: {r.stderr}")
        return r.stdout.strip()

    def write(self, path, text):
        full = os.path.join(self.root, path)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w", encoding="utf-8") as f:
            f.write(text)

    def read(self, path):
        with open(os.path.join(self.root, path), encoding="utf-8") as f:
            return f.read()

    def commit(self, msg):
        self.git("add", "-A")
        self.git("commit", "-q", "-m", msg)

    def rfc(self, *args):
        r = subprocess.run([sys.executable, RFC, *args], cwd=self.root, capture_output=True,
                           text=True, env=self.env)
        return r.returncode, r.stdout.strip(), r.stderr.strip()

    def reserve_branch(self, slug="three", title="Three"):
        self.git("checkout", "-q", "-b", f"rfc/{slug}", "master")
        code, out, err = self.rfc("reserve", slug, title, "--author", "fox", "--date", "2026-10-23")
        assert code == 0, err
        self.commit(f"reserve {slug}")
        self.git("checkout", "-q", "master")
        return out


class RfcTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.repo = Repo(self.tmp.name)

    def tearDown(self):
        self.tmp.cleanup()

    def test_next_takes_the_highest_of_files_rows_and_next_line(self):
        self.assertEqual(self.repo.rfc("next")[1], "0003")
        self.repo.write("docs/rfcs/0007-seven.md", "# RFC 0007: Seven\n")
        self.assertEqual(self.repo.rfc("next")[1], "0008")
        self.assertEqual(self.repo.rfc("next", "--min", "41")[1], "0041")

    def test_reserve_writes_stub_row_and_next_number(self):
        code, out, err = self.repo.rfc("reserve", "three", "Three Things", "--author", "fox",
                                       "--date", "2026-10-23")
        self.assertEqual(code, 0, err)
        self.assertEqual(out, "docs/rfcs/0003-three.md")
        stub = self.repo.read(out)
        self.assertTrue(stub.startswith("# RFC 0003: Three Things\n"))
        self.assertIn("**Status**: draft", stub)
        readme = self.repo.read("docs/rfcs/README.md").splitlines()
        row = "| 0003 | [Three Things](0003-three.md) | draft | |"
        self.assertEqual(readme[readme.index(row) - 1], "| 0002 | [Two](0002-two.md) | draft | |")
        self.assertIn("Next number: **0004**", readme)
        self.assertEqual(self.repo.rfc("check")[0], 0)

    def test_reserve_rejects_a_bad_slug(self):
        self.assertEqual(self.repo.rfc("reserve", "Bad Slug", "T")[0], 2)

    def test_check_flags_an_unlisted_duplicate_and_a_stale_next_number(self):
        self.repo.write("docs/rfcs/0002-other.md", "# RFC 0002: Other\n")
        self.repo.write("docs/rfcs/0005-five.md", "# RFC 0005: Five\n")
        code, _, err = self.repo.rfc("check")
        self.assertEqual(code, 1)
        self.assertIn("0002 is used by", err)
        self.assertIn("Next number is 0003 but 0005 is taken", err)

    def test_check_allows_a_listed_collision(self):
        self.repo.write("docs/rfcs/0002-other.md", "# RFC 0002: Other\n")
        self.repo.write("docs/rfcs/README.md", README + "| 0002 | Two | Other |\n")
        self.assertEqual(self.repo.rfc("check"), (0, "", ""))

    def test_verify_accepts_a_reservation(self):
        self.repo.reserve_branch()
        code, out, err = self.repo.rfc("verify-reservation", "master", "rfc/three")
        self.assertEqual((code, out), (0, "reservation 0003"), err)

    def test_verify_skips_ordinary_prs(self):
        self.repo.git("checkout", "-q", "-b", "feature", "master")
        self.repo.write("src/main.go", "package main // changed\n")
        self.repo.commit("code")
        code, out, _ = self.repo.rfc("verify-reservation", "master", "feature")
        self.assertEqual(code, 3)
        self.assertIn("not a reservation", out)

    def test_verify_skips_a_written_rfc(self):
        self.repo.git("checkout", "-q", "-b", "full", "master")
        self.repo.write("docs/rfcs/0003-three.md", "# RFC 0003: Three\n\n" + "body\n" * 40)
        self.repo.commit("full rfc")
        self.assertEqual(self.repo.rfc("verify-reservation", "master", "full")[0], 3)

    def test_verify_skips_a_reservation_with_other_changes(self):
        self.repo.git("checkout", "-q", "-b", "rfc/three", "master")
        self.repo.rfc("reserve", "three", "Three")
        self.repo.write("src/main.go", "package main // sneaky\n")
        self.repo.commit("reserve plus code")
        code, out, _ = self.repo.rfc("verify-reservation", "master", "rfc/three")
        self.assertEqual(code, 3)
        self.assertIn("src/main.go", out)

    def test_verify_rejects_a_number_master_took_meanwhile(self):
        self.repo.reserve_branch("three", "Three")
        self.repo.write("docs/rfcs/0003-other.md", "# RFC 0003: Other\n")
        self.repo.commit("someone else took 0003")
        code, _, err = self.repo.rfc("verify-reservation", "master", "rfc/three")
        self.assertEqual(code, 1)
        self.assertIn("0003 is already taken", err)

    def test_verify_rejects_extra_readme_edits(self):
        self.repo.git("checkout", "-q", "-b", "rfc/three", "master")
        self.repo.rfc("reserve", "three", "Three")
        readme = self.repo.read("docs/rfcs/README.md").replace("| draft | |", "| accepted | |")
        self.repo.write("docs/rfcs/README.md", readme)
        self.repo.commit("reserve and flip a status")
        code, _, err = self.repo.rfc("verify-reservation", "master", "rfc/three")
        self.assertEqual(code, 1)
        self.assertIn("may change only the new row", err)

    def test_index_is_current_after_a_reservation(self):
        self.assertEqual(self.repo.rfc("index"), (0, "", ""))
        self.repo.rfc("reserve", "three", "Three", "--author", "fox", "--date", "2026-10-23")
        self.assertEqual(self.repo.rfc("index"), (0, "", ""))

    def test_index_regenerates_rows_from_the_headers(self):
        self.repo.write("docs/rfcs/0002-two.md",
                        "# RFC 0002: Two, Renamed\n\n**Status**: accepted; phase 1 built\n"
                        "**Author**: fox\n**Summary**: Does a | b\n\n## Problem\n\n**Summary**: no\n")
        self.repo.write("docs/rfcs/0003-three.md", "# RFC 0003: Three\n\n**Status:** Paused (2026-10-23)\n")
        code, out, _ = self.repo.rfc("index")
        self.assertEqual((code, out), (1, "docs/rfcs/README.md"))
        self.assertEqual(self.repo.rfc("index", "--write")[:2], (0, "docs/rfcs/README.md"))
        readme = self.repo.read("docs/rfcs/README.md")
        self.assertIn("| 0001 | [One](0001-one.md) | accepted | |\n"
                      "| 0002 | [Two, Renamed](0002-two.md) | accepted | Does a \\| b |\n"
                      "| 0003 | [Three](0003-three.md) | paused | |\n\n"
                      "Next number: **0003**\n", readme)
        self.assertEqual(self.repo.rfc("index"), (0, "", ""))

    def test_index_regenerates_another_scopes_table_under_its_heading(self):
        self.repo.write("plane/docs/rfcs/0001-wings.md",
                        "---\nstatus: implemented\nauthor: agent\n---\n\n# RFC 0001: Wings\n")
        self.repo.write("plane/docs/rfcs/002-tail.md",
                        "# RFC: Tail\n\n- **Status**: draft\n")
        self.repo.write("plane/docs/rfcs/0003-nose.md",
                        "# RFC 0003: Nose\n\n## Status\n\nRejected (2026-10-01)\n\n## Summary\n")
        self.repo.write("plane/docs/rfcs/README.md", "# Plane\n\n| RFC | Title | Status |\n|-----|-------|--------|\n")
        self.repo.write("docs/rfcs/README.md", README + textwrap.dedent("""\
            ## Project Scopes

            ### plane (`plane/docs/rfcs/`)

            | RFC | Title | Status |
            |-----|-------|--------|
            | 0001 | [Old](../../plane/docs/rfcs/0001-wings.md) | draft |
            """))
        self.repo.rfc("index", "--write")
        rows = ("| 0001 | [Wings]({p}0001-wings.md) | implemented |\n"
                "| 002 | [Tail]({p}002-tail.md) | draft |\n"
                "| 0003 | [Nose]({p}0003-nose.md) | rejected |\n")
        self.assertTrue(self.repo.read("docs/rfcs/README.md").endswith(
            rows.format(p="../../plane/docs/rfcs/")))
        self.assertTrue(self.repo.read("plane/docs/rfcs/README.md").endswith(rows.format(p="")))
        # The root scope's own table is still the first one.
        self.assertIn("| 0002 | [Two](0002-two.md) | draft | |", self.repo.read("docs/rfcs/README.md"))

    def test_verify_index_accepts_a_regeneration_and_skips_other_prs(self):
        self.repo.write("docs/rfcs/0002-two.md", "# RFC 0002: Two\n\n**Status**: accepted\n")
        self.repo.commit("accept two")
        self.repo.git("checkout", "-q", "-b", "rfc-index/regenerate", "master")
        self.repo.rfc("index", "--write")
        self.repo.commit("regenerate")
        code, out, err = self.repo.rfc("verify-index", "master", "rfc-index/regenerate")
        self.assertEqual((code, out), (0, "index regeneration"), err)
        self.assertEqual(self.repo.rfc("verify-index", "master", "master")[0], 3)

        self.repo.write("docs/rfcs/README.md",
                        self.repo.read("docs/rfcs/README.md").replace("| accepted | |", "| rejected | |", 1))
        self.repo.commit("hand edit")
        code, _, err = self.repo.rfc("verify-index", "master", "rfc-index/regenerate")
        self.assertEqual(code, 1)
        self.assertIn("not what scripts/rfc index generates", err)

        self.repo.write("src/main.go", "package main // sneaky\n")
        self.repo.commit("code")
        self.assertEqual(self.repo.rfc("verify-index", "master", "rfc-index/regenerate")[0], 3)


if __name__ == "__main__":
    unittest.main()
