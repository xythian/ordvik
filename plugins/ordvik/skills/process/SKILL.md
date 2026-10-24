---
name: process
description: The development process ordvik repos follow, from a goal to a merged PR. Use when starting new work, deciding whether a change needs research, an RFC, a plan or a dispatch, deciding what to ask the operator, driving a PR to merge, or closing a thread.
allowed_tools:
  - Read
  - Glob
  - Grep
---

# Development Process

How work moves from an idea to merged code in a repo that uses ordvik. The
other skills hold the formats (`/ordvik:rfc`, `/ordvik:dispatch`,
`/ordvik:planning`, `/ordvik:asks`); this one says when each is used, who
decides what, and how a thread finishes. A repo binds it to its own hosts,
test commands and default branch in its `AGENTS.md`; where the two differ,
the repo's `AGENTS.md` wins.

**The operator** is the person who owns the repo. **A thread** is one agent
session working one piece of work, usually on its own branch and PR.

## The operator's time

The operator's attention is the scarce resource, so the process spends it
only on judgement and decisions. Everything routine runs on a machine.

- Ask the operator only to **decide** (a choice, a priority, an RFC's
  acceptance), to **review** (a PR), to **approve** (a dry run a thread
  then runs with `--commit`), or to **do** what needs their own hands,
  devices or accounts and no command covers.
- **Automate operator steps; don't hand them over.** A list of commands
  for the operator to type is a missing command. Build it: dry run by
  default, `--commit` to act, every check that changes nothing run before
  the first change, a clear stop when something needs attention, and a
  safe re-run after a stop.
- **The operator approves; a thread runs it.** A production step built
  from existing commands is an approval gate. The thread shows the dry
  run, the operator approves it, and the thread runs the `--commit` on a
  venue that holds the authority (below). Word such a gate as an approval
  and name the command.
- Every open ask is recorded in git where it belongs (a dispatch's human
  gate, a plan gate, an RFC at `considering`, or an ask file), so
  `scripts/asks` can list them all. A kind of **do** ask that keeps
  coming back is a backlog item to automate.
- Ask each question once, with a recommendation, and keep working on the
  recommended option while waiting unless the step cannot be undone.

## Stages

```
goal ─▶ research ─▶ RFC ─▶ plan and dispatches ─▶ implement ─▶ PR ─▶ close
  ▲                                                                   │
  └──────────────────────── weekly planning ◀─────────────────────────┘
```

### 1. Goal and design

Iterate with the operator on what the work is for and roughly how it
should go, in the thread's conversation. Settle scope, what is out of
scope, and what "done" means before any research or drafting.

The conversation alone is enough when it only shaped the goal. When
research that changed a decision came out of it, that research goes into
git (stage 2).

### 2. Research

Research that drove a decision is recorded: a standalone doc under the
repo's research directory when it outlives one RFC, or an RFC appendix when
it only serves that RFC (`/ordvik:research` makes the call). Research that
changed nothing stays in the thread.

### 3. RFC

**When an RFC is needed.**

| Change | RFC |
|---|---|
| Bug fix, minor change inside one service | No |
| Larger change inside one service | When it changes how the service works in a way others should review; the operator's call |
| A contract between services (API, data format, protocol) | Yes |
| A major feature spanning several subcomponents | Yes |
| A new dependency (library, framework, external service) | Yes, and the operator approves the dependency |
| A production procedure (deploy, migration, restore, host move) | Yes |

**Reserve, then write.** Where the repo has `scripts/rfc`, reserve the
number first with a reservation PR that merges itself, then write the RFC on
its own PR (`/ordvik:rfc reserve`). The RFC's PR touches only its file and
its own index row.

**Merging the RFC's PR accepts it.** An RFC is written to be accepted, so
its PR lands it as `accepted` and the merge is the operator's acceptance.
An RFC the operator tables to keep work in progress lands as `draft` and its
PR says it is tabled. `accepted` stays through the rollout; `implemented`
means the product shipped.

### 4. Plan and dispatches

Work goes to threads through dispatch documents (`/ordvik:dispatch`).
Each has a goal, context, numbered tasks with automated validation,
completion criteria, and human gates worded as above.

- **Names, not numbers.** A new dispatch is `YYYYMMDD-<slug>.md`, and that
  name is its thread id. Older numbered docs keep their numbers.
  `scripts/plan check` fails when two active docs share an id.
- **One thread or a plan.** Work that fits one round of threads is one or
  more standalone dispatches. Work that spans several rounds gets a plan
  (`docs/planning/plans/<slug>.md`, see `/ordvik:dispatch` and its
  `references/plans.md`): its threads, what each waits on, its gates and a
  log. A thread launches as soon as what it waits on has merged.
- **A plan has one conductor.** One agent session owns the plan until it
  closes: it launches threads as they become ready, writes later dispatch
  docs from what earlier threads learned, asks the operator at gates, and
  makes every change to the plan. The operator steers the plan by talking
  to the conductor.
- Threads talk to each other only when one's work depends on the other's.
  A thread others wait on lands those tasks in their own PRs and tells the
  conductor when each merges.

### 5. Implement

A thread works on its own branch, commits often, and runs the repo's fast
tests before each push. It records what it finds outside its scope under
"Pre-existing issues" in its dispatch doc rather than fixing it.

**Where work runs.** Each venue has an identity, and work runs on the
least-privileged venue that can do it. A step that needs more authority
than its venue has becomes an approval, not a command for the operator.

| Venue | Typical identity | Runs |
|---|---|---|
| Cloud session | Repo access only; no network secrets | Design, RFCs, code, tests, PRs |
| Agent workstation | Tailnet and test infrastructure, no production secrets | Integration suites, long builds, launching threads |
| Ops agent | Production authority inside an approved window | `--commit` runs the operator approved |
| The operator's machine, by remote control | The operator's own accounts | Only what no agent venue can do yet; a fallback once an ops agent exists |
| The operator by hand | Devices, physical access, accounts with no API | Hardware, first-time account setup |

The repo's `AGENTS.md` names its actual hosts for each row.

### 6. PR

- **One PR per thread**, plus one per task that other threads wait on.
- **The thread drives its PR to mergeable.** It keeps CI green, rebases
  when the default branch moves under it, answers or fixes every review
  comment, and tells the operator once when it is ready or needs them.
- **History is flat.** Rebase onto the default branch; no merge commits.
  Before review, the thread tidies its commits into logical, buildable
  steps and force-pushes its own branch, without asking. It never rewrites
  anyone else's branch.
- **Who merges.** The operator reviews and merges code. PRs that change
  only docs or planning files (dispatch docs, plans, backlog, status, RFC
  reservations) merge themselves once CI is green, where the repo has a
  workflow for it; until then the operator merges them too. Merging some code on an
  automated risk assessment is future work and needs its own RFC.
- **Attribution.** Commits are authored by the operator. Follow the repo's
  attribution rule exactly; where it forbids AI attribution, set the git
  identity to the operator before the first commit and remove any footer a
  tool adds to a PR body.

### 7. Close

- **A dispatch closes in its own PR**, as the last commit before review:
  `status: done`, the doc moved to `dispatch/archive/`, links fixed, the
  thread registry row updated. Merging the PR is what completes the thread.
- **Retros.** A plan writes a retro when it closes. A large standalone
  thread (about five or more tasks, or a week or more) writes a short one
  in its dispatch doc's `## Retro`. A fix that took several failed
  attempts gets a TIL.
- If a human gate is still open when the PR goes up, ask the operator
  whether to close the dispatch now or leave it `active` until the gate is
  done.

## Planning cadence

A planning session (`/ordvik:planning`) gathers plan status and the asks
queue, gardens stale dispatches and gates, reviews the backlog, records
the result as one PR, and offers the operator at most five starts. The
repo sets the schedule (for example, a full pass weekly and a light asks
check mid-week). Plans keep their conductors: planning reads plan status
and never launches a plan's threads itself.

## Git is the record

Everything that has to outlive a conversation lives in the repo: RFCs,
research that drove decisions, dispatch docs, plans and their logs, asks,
the backlog, retros and TILs. Who is running what lives with the
conductor and in the PRs.
