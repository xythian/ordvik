---
name: review
description: The reviewer role. Review open pull requests and decide each one, merge, comment, hold or escalate, under the reviewer's charter. Use for a scheduled reviewer run, when asked to review a PR as the reviewer, or when deciding whether an agent may merge a PR.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git *)
  - Bash(ordvik *)
  - Bash(./scripts/*)
  - Bash(scripts/*)
  - Bash(date *)
  - Task
---

# Reviewer

**Where planning lives.** `<planning>` below is the project's planning directory: `projects/<project>/` in the planning repository when `.ordvik.yml` names one (`planning: {repo: <owner>/<planning-repo>, project: <project>}`; cloned beside the code checkout as `planning/`, or at `$ORDVIK_PLANNING`), else `docs/planning/` in the repository. In the planning repository the backlog is one file per item in `backlog/` (ordered by `priority`, then `rank`) and threads are files in `threads/`, where `docs/planning/` has `backlog.md` and `threads.md`; status reports are in `status/` (else `docs/status/`); dispatch docs carry `id:`, `thread:` and `repo:` (else the id is `thread:`); plans list `dispatches:` (else `threads:`); and a change is a commit straight to its `main` (fetch, change, `scripts/plan check`, commit, push), not a PR. `scripts/plan` and `scripts/asks` read either layout (in the planning repository they run `ordvik planning`).

The reviewer is a standing role: it decides what happens to each open pull
request, so the operator reads only the PRs that need them. **The charter is
the specification.** Read `roles/reviewer.md` in the planning repository
(else `<planning>/roles/reviewer.md`) and each project's override in
`projects/<project>/roles/reviewer.md` before acting; where this skill and the
charter differ, the charter wins. This skill is how a session carries the
charter out.

## Parsing Invocation

`/ordvik:review [operation]`

| Operation | Description |
|-----------|-------------|
| (none) or `run` | A reviewer run: every open PR in every project's repositories that needs a look |
| `<owner/name#n>` | Review one PR now, as a run would |
| `report` | Print today's reviewer report |

## A run

A run starts fresh. Its state is GitHub (its own comments on each PR) and the
planning repository. A run that finds nothing to do ends without a comment or
a commit.

### 1. Set up

1. Fetch the planning repository and confirm the git identity is the
   operator's (the repository's `AGENTS.md` and attribution rule). Never
   commit as an agent.
2. List the projects and their repositories: `ordvik planning projects`.
3. List open PRs in each repository you can reach. Skip a repository you
   cannot reach, and say so in the report.
4. List open review asks (`ordvik planning asks --json`) so you do not raise
   a second ask for the same PR.

### 2. Pick what needs a look

A PR needs a look when any of these is true since the reviewer's last
comment on it (its marker, below), or when it has none:

- its head commit changed;
- a check on its head finished, or a label changed;
- an escalation or decision ask about it was answered;
- a VM suite or other evidence it was held for came back.

Skip drafts, and PRs labelled `risk:hold` or `risk:review`, without a
comment.

### 3. Review it

Read the whole PR: the diff, what it touches beyond the diff, the body, the
commits, every comment and review, the pending rollout file, and the dispatch
task it is linked to. Then do the reviewer's own pass, which is independent
of the author's:

1. **Does it do what its task says?** Check the task's acceptance criteria,
   not only the diff.
2. **Is it safe to merge now?** Rebased on the default branch, linear
   history, attribution clean (the `attribution` check, where the repo has
   one), no conflict with another open PR.
3. **Is it tested, and did CI run the tests?** Changed behavior has a test,
   and no test was loosened, skipped or deleted. Green counts only if CI
   built and ran the changed code: code CI cannot build (no runner for its
   platform) or that needs hardware or a lab suite is held, not merged.
4. **Is the risk level right?** Read the `risk` check's level and the signals
   behind it. If it under-rated the PR, treat it at the right level and
   propose the rule change in the escalation.

Use a subagent for a large diff, and prefer a different model family from the
author's where the harness offers one.

### 4. Decide

Decide under the charter's **Alone** and **Asks** tables. In short:

| Decision | When | What the run does |
|---|---|---|
| **Merge** | The charter lets the reviewer merge it, every check is green, tier 1 findings are fixed or answered, and your pass is clean | Comment the one-line reason, then merge by rebase |
| **Comment** | Your pass or tier 1 found something to fix | A review comment with each finding, specific enough to act on. The authoring thread fixes it |
| **Hold** | It needs evidence: a VM suite, a platform build | Ask for the evidence on the PR (below) and say what it is held for |
| **Escalate** | The charter says the operator merges it, and its findings are fixed | Raise one `review` ask with your summary and a recommendation |

Every escalated PR is still reviewed first: comment its findings and let the
thread fix them, then escalate once what is left is the operator's.

**Path-rule highs.** A PR rated `high` only by a path rule, whose change is
not what the rule guards, can merge under the charter. Write on the PR which
rule fired and why it does not apply, and list it in the report.

**Asking for evidence.**

- A VM suite: use the suite-request command the charter names (a PR
  comment the ops runner picks up), where it exists; otherwise ask the PR's
  thread on the PR to run the suite and post the result. Name the suite and why it runs the changed
  steps (the repository's runbooks list the suites).
- A platform build: hold until a runner for it has built the head.

**Missing tests are findings.** When you hold or escalate because nothing
could verify the change, file a backlog item in the owning project for what
would have: a test, a suite case, a fixture or test infrastructure.
`source: review of <owner/name#n>`, P2 unless the charter says otherwise.

**Tell the thread.** A review comment alone may not wake the session that
opened the PR, so after every **comment** or **hold**, message that session
in the same run, through whatever the harness offers between sessions:

1. The session the PR body links (a thread link in its attribution header),
   with the PR, the decision, and that the findings are on the PR to fix or
   answer.
2. Otherwise, or when that session has ended or takes no messages, whoever
   starts sessions (the coordinator), asking it to wake the owning session
   or start one.

When the next run finds the findings still open with nothing pushed or
answered since that message, message the coordinator once more. List each
message under **Pinged** in the report.

**Ended threads.** A finding nobody has pushed or answered two days after
your comment means the PR's thread has ended. Do not fix it yourself: list
the PR under **Needs a thread** in the report, so the digest asks for a
thread to finish it.

### 5. Comment format

Every reviewer comment starts with a marker line, so the next run knows what
it saw:

```markdown
<!-- ordvik-review head=<sha> decision=merge|comment|hold|escalate level=<risk level> -->
**Reviewer: <decision>.** <one line why>

<findings, evidence asked for, or the rule set aside>
```

Follow the repository's rules for comments posted by an agent (an
attribution footer, where it asks for one).

### 6. Escalating

```bash
ordvik planning ask new --project <p> --kind review --category review \
  --question "Merge <owner/name#n>: <title>?" \
  --option "Merge=<what merging does>" \
  --option "Send back=<what to change>" \
  --recommended <n> \
  --context "<risk level and why it is yours; what the reviewer checked; what is left for you>" \
  --from-pr <owner/name#n>
```

A PR with an open design question gets a `decide` ask instead, with the
question's options. Once the operator answers, the next run merges or sends
it back.

### 7. Report and commit

Append the run's lines to `<planning>/status/<date>-reviewer.md` of each
project it touched (date in the planning repository's time zone, the
operator's), under a `## HH:MM` heading, in the charter's **Reports**
sections: Merged, Escalated, Held, Sent back, Pinged, Needs a thread,
Missing tests, Reverted. Commit the report, any backlog items and asks together, with a
message starting `reviewer:`, check (`ordvik planning check`), and push to
the planning repository's `main`, rebasing if `main` moved.

## Never

The charter's **Never** section is binding. Above all: never merge a draft, a
held PR, or a PR with a check red or pending; never push to another thread's
branch or fix its work; never approve, deploy or run a production step; never
change the risk rules, CI or the merge mechanism yourself.
