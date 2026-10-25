---
name: asks
description: Operator asks: list everything waiting on the human operator (decisions, reviews, steps only they can do), sweep in open PRs and blocked threads, record an ask with no other home, and answer one. Use when the user asks what needs them, what they are blocking, or what to decide, review or do.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git log *)
  - Bash(git mv *)
  - Bash(git status *)
  - Bash(./scripts/asks *)
  - Bash(scripts/asks *)
  - Bash(date *)
  - AskUserQuestion
---

# Operator Asks Skill

**Where planning lives.** `<planning>` below is the project's planning directory: `projects/<project>/` in the planning repository when `.ordvik.yml` names one (`planning: {repo: <owner>/<planning-repo>, project: <project>}`; cloned beside the code checkout as `planning/`, or at `$ORDVIK_PLANNING`), else `docs/planning/` in the repository. In the planning repository the backlog is one file per item in `backlog/` (ordered by `priority`, then `rank`) and threads are files in `threads/`, where `docs/planning/` has `backlog.md` and `threads.md`; status reports are in `status/` (else `docs/status/`); dispatch docs carry `id:`, `thread:` and `repo:` (else the id is `thread:`); plans list `dispatches:` (else `threads:`); and a change is a commit straight to its `main` (fetch, change, `scripts/plan check`, commit, push), not a PR. `scripts/plan` and `scripts/asks` read either layout (in the planning repository they run `ordvik planning`).

An **ask** is something only the operator can move: a decision, a review,
or a step that needs their accounts, devices or approval. Asks are already
recorded across the repo (human gates, plan gates, RFCs awaiting a
decision) and outside it (open PRs, blocked agent threads). This skill
puts them in one list so that nothing waiting on the operator gets
missed, and it turns kinds of steps that keep recurring into candidates
for automation.

Read `references/asks.md` for the full design: what counts, where asks
come from, the ask file format, and how recurring asks become commands.

Requires `scripts/asks` (and `scripts/plan`, which it imports) in the
repo. If they are missing, offer to copy both from
`${CLAUDE_PLUGIN_ROOT}/scripts/` and make them executable (they need git
and PyYAML).

## Parsing Invocation

`/ordvik:asks [operation]`

| Operation | Description |
|-----------|-------------|
| (none) or `status` | Run `scripts/asks status --fetch` and summarize it |
| `sweep` | The full queue: git asks plus open PRs and blocked threads |
| `add <description>` | Record an ask that has no doc to live in |
| `answer <slug>` | Record the answer to an ask file and archive it |

## status

Run `scripts/asks status --fetch`, or add `--all` when the user wants RFC
drafts that have gone idle. Lead with the asks that unblock the most, and
point out gates marked `*`, whose earlier tasks are not marked done:
either the gate isn't reachable yet, or the dispatch is stale.

## sweep

1. Run `scripts/asks status --fetch --json`.
2. List the open PRs on the repo, and on any repo that dispatch docs
   target. Find the ones waiting on the operator to review or merge, each
   with its CI state. A draft RFC PR counts as a `review` ask.
3. If the session can see agent threads (for example, a Claude project),
   list the threads that are blocked or ready for review, with what each
   needs in one line. An unanswered decision card counts as a `decide`
   ask.
4. Merge the lists. When an ask appears in more than one place, list it
   once and give every link.
5. Report the asks grouped as Decide, Review and Do, most unblocking first.
   Put the asks that can start now ahead of the ones that need a merge
   first. An `approve` ask is a dry run for the operator to OK, which an
   agent then runs. Don't present it as commands for them to type. End with stale gates and candidates to automate. Keep the
   report to a page; a doc or file is better than a long chat message.

The sweep changes nothing on its own. Closing stale gates, archiving
answered asks and adding automation backlog items go through a planning
session (`/ordvik:planning`) or a PR the operator asks for.

## add

Only for an ask that has nowhere else to live. A human gate belongs in its
dispatch doc's `human_gates` and Task heading. A plan gate belongs in the
plan's `gates`. If one of those fits, put the ask there instead.

Write `<planning>/asks/<slug>.md`:

```yaml
---
ask: <one line, verb first>
kind: do                     # decide | review | approve | do
category: <word>             # optional: tailnet-policy, restore-drill, files, ...
from: "<thread, PR or doc that raised it>"
unblocks: <what moves once it is answered>   # optional
raised: YYYY-MM-DD
defer_until: YYYY-MM-DD      # optional
---

Context: what is needed, why, and links.
```

Run `scripts/asks check` before committing.

## answer

Add `answered: YYYY-MM-DD`, `outcome:` (`done`, `decided`, `rejected`,
`automate` or `dropped`) and `answer:` (one line, with a link to whatever
records the result). Then `git mv` the file to `<planning>/asks/archive/`.
For `automate`, the answer names the backlog item or dispatch that will
build the command. Run `scripts/asks check`.

For a dispatch gate, the answer is to mark its Task heading `— DONE` (and
record the result in the task). For a plan gate, set `passed:` through the
plan's conductor. For an RFC, change its status.

## Turning recurring asks into commands

`scripts/asks status` ends with candidates to automate: every category
with three or more open `do` asks. For each one, propose one of two
answers to the operator:

- **Automate**: a backlog item for a command that does the step. It runs a
  dry run by default and acts with `--commit`, so the operator approves a
  diff instead of typing commands.
- **Keep**: the step really needs a person. Record why in the planning
  report, so the same category isn't raised again every week.
