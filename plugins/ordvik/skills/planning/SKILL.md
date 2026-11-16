---
name: planning
description: Run a planning session — review threads, triage backlog, plan and close dispatch plans, and generate a planning report. Use when the user asks for a planning review, weekly planning, or wants to assess project status.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git log *)
  - Bash(git diff *)
  - Bash(git status *)
  - Bash(git branch *)
  - Bash(date *)
  - Bash(python *)
  - Task
  - AskUserQuestion
---

# Planning Session Skill

**Where planning lives.** `<planning>` below is the project's planning directory: `projects/<project>/` in the planning repository when `.ordvik.yml` names one (`planning: {repo: <owner>/<planning-repo>, project: <project>}`; cloned beside the code checkout as `planning/`, or at `$ORDVIK_PLANNING`), else `docs/planning/` in the repository. In the planning repository the backlog is one file per item in `backlog/` (ordered by `priority`, then `rank`) and threads are files in `threads/`, where `docs/planning/` has `backlog.md` and `threads.md`; status reports are in `status/` (else `docs/status/`); dispatch docs carry `id:`, `thread:` and `repo:` (else the id is `thread:`); plans list `dispatches:` (else `threads:`); and a change is a commit straight to its `main` (fetch, change, `scripts/plan check`, commit, push), not a PR. `scripts/plan` and `scripts/asks` read either layout (in the planning repository they run `ordvik planning`).

Runs a planning session: reviews all threads, triages the backlog, identifies ready work, and generates a planning report. This is the orchestration layer that connects threads, backlog, dispatch docs, and dispatch plans.

Requires the planning structure (`<planning>/`). If it doesn't exist, suggest running `/ordvik:init` first.

## Parsing Invocation

The user invokes this skill as `/ordvik:planning [focus]`.

- Without arguments: full planning session (all threads, full backlog review)
- With a focus: targeted session (e.g., `/ordvik:planning plan-close bootstrap-drift`, `/ordvik:planning replan`)

| Focus | Description |
|-------|-------------|
| (none) | Full planning session |
| `plan-close <slug>` | Close out a finished dispatch plan — update backlog, threads, write retro |
| `replan` | Replanning session — assess what changed and adjust |
| `dispatch-ready` | Review backlog for items ready to dispatch |
| `digest` | Write the daily digest from every role's report and the asks queue |

## Full Planning Session

### Step 1: Read Planning Artifacts

Read the authoritative sources:

1. `<planning>/threads.md` — the thread registry
2. `<planning>/backlog.md` — prioritized work items
3. `.ordvik.yml` — scope registry (if multi-scope)
4. `TODO.md` — untriaged items (if it exists)

**Every thread in the registry must appear in the report.** No silent drops — if a thread has no activity, say so explicitly.

### Step 2: Gather Current State

For each thread, gather data appropriate to its status:

**Active threads** — full review:
1. Recent commits: `git log --oneline --since="2 weeks ago" -- {paths}`
2. RFC status: read the `**Status**` line of each listed RFC
3. Branch activity: check for in-progress feature branches
4. Backlog items: check priority and task-readiness
5. Active dispatch docs: check `<planning>/dispatch/` for in-progress work

**Stable/dormant/parked threads** — lighter review:
1. Recent commits: same check, expect "None" — flag unexpected activity
2. Brief note on what would trigger reactivation

Use Task/Explore agents to parallelize research across many threads. Thoroughness matters more than speed.

### Step 2b: Sweep Operator Asks

If `scripts/asks` exists, run the sweep (`/ordvik:asks sweep`): everything
waiting on the operator, from dispatch gates, plan gates, RFCs awaiting a
decision, ask files, open PRs and blocked agent threads. Carry it into the
report's "Waiting on the Operator" section. While doing so:

- Propose closing stale gates (dispatch tasks that shipped but were never
  marked done, or dispatches whose work was superseded). The operator
  confirms each one.
- For each candidate to automate, propose **Automate** (a backlog item for
  the command) or **Keep** (with the reason).

Read ages from full history: in a shallow clone (`git rev-parse
--is-shallow-repository`) run `git fetch --unshallow origin` first, or every
gate looks as old as the clone.

### Step 2c: Rebuild the Stall Register

In the planning repository, rebuild `<planning>/stalled.md` (its rules are
the planning repository's `roles/README.md`, "The stall register") from
Steps 2 and 2b: every gate or ask open 7 days, merged work waiting on a
release, dispatches `ready` for 2 days with no launch request, plans and
dispatches finished but still active, open PRs idle 3 days (holds
included), and blocked or unanswered review-ready threads. For each, write
**why** it stopped and **what unblocks it**, naming who, and group rows by
the blocker they share so one action that clears many shows as one line.

- Triage every gate three weeks old into *ride the next train*, *one
  approved run*, or *close / re-scope*, and say which.
- Request a launch (`ordvik planning launch`) for each ready dispatch with
  nothing in its way, or say in the register why not.
- Carry the register's blockers, most items cleared first, into the
  report's "Stalled" section.

### Step 3: Triage TODO.md

If `TODO.md` exists, triage each item:
- Assign to an existing thread, or identify it as a new thread
- Add to backlog at appropriate priority
- Remove triaged items from TODO.md

### Step 4: Generate Planning Report

Structure:

```markdown
## HH:MM - Planning Session: [Date]

### Active Threads

#### [Thread Name] — [One-line Description]
**Current state**: What's implemented, what phase
**Recent activity**: Commits in last 2 weeks, or "None"
**Backlog**: P0/P1 items
**Next steps**: Concrete, actionable items

---

### Stable Threads
[Brief status for each]

### Parked/Dormant Threads
[Brief status, what would reactivate]

---

### Priorities

1. [Highest priority with justification — reference backlog items]
2. ...

### Ready to Dispatch

Items that are task-ready or could be made task-ready this session:
- [Backlog item] — [what's needed to dispatch]

### Consciously Deferred

- [Thread/item] — [reason, what would reactivate]

### Stalled

[From the stall register: each shared blocker, how many items it holds and
for how long, and what clears it]

### Waiting on the Operator

**Decide**: [ask] — [what it unblocks] ([link])
**Review**: [PR or check] — [state] ([link])
**Do**: [step] — [can start now / after what] ([link])
**Stale gates**: [gate] — [proposed close or keep]
**Candidates to automate**: [category] ([count]) — Automate: [backlog item] / Keep: [reason]
```

Key principles:
- **Every registry thread gets a section** — no silent drops
- **Be specific about next steps** — actionable, not vague
- **"Recent activity: None"** makes stale threads visible
- **"Consciously Deferred"** forces explicit acknowledgment
- **"Waiting on the Operator"** means nothing that needs the human is buried in a thread

### Step 5: Route the Report

Use the same output routing as `/ordvik:status`, with suffix `planning`:
- Local: `<planning>/status/YYYY-MM-DD-planning.md` in the planning repository, else `docs/status/YYYY-MM-DD-planning.md`
- Obsidian: `projects/{project}/YYYY-MM-DD-{project}-planning`

### Step 6: Update Planning Documents

After generating the report, update if needed:

**`<planning>/threads.md`** — if:
- New threads were discovered
- A thread's status changed
- The user provided corrections

**`<planning>/backlog.md`** — if:
- Items were completed
- Priorities need adjusting
- New items emerged
- Dependencies changed

Always tell the user what document changes you made.

---

## Plan Close-Out

`/ordvik:planning plan-close <slug>`

1. Run `scripts/plan status <slug> --fetch`. Every thread must be `done`,
   `superseded` or `dropped`; if not, report what remains and stop.
2. Check each thread was closed per `/ordvik:dispatch close` (status,
   `— DONE` markers, completion criteria, archived).
3. Update backlog — mark completed items with date, note follow-ups
4. Update thread registry if thread statuses changed
5. Write the retro in the plan's **Retro** section (format below)
6. Set the plan's `status: done` and move it to `<planning>/plans/archive/`

Note: Thread-level format concerns (status fields, DONE markers, supersession, archive) are owned by the dispatch skill and its conventions doc; the plan format and conductor loop are in the dispatch skill's `references/plans.md`.

Retro format:

```markdown
## Retro

**Date**: YYYY-MM-DD
**Threads**: [list]

### Completed
- [Thread] — [summary of what shipped]

### Still Open
- [Thread] — [what remains, why]

### Scope Changes
- [What changed from original dispatch and why]

### Unblocked
- [What's now ready for follow-on work]

### Observations
- What worked well
- What didn't
- Process improvements for the next plan
```

## Replanning

`/ordvik:planning replan`

1. Assess current state — what changed since the last plan?
2. Review in-progress dispatch docs for scope changes
3. **Check for stale dispatch status** — Identify dispatch docs with `status: active` (or missing status) where work is clearly done or superseded. Flag these for close-out.
4. Identify blocked or delayed threads
5. Update dispatch docs (add tasks, mark descoped items with reason)
6. For dispatch plans in `<planning>/plans/`, run `scripts/plan status <slug> --fetch` and route plan changes through the plan's conductor (`/ordvik:dispatch plan amend`)
7. Update backlog priorities
8. Record the decision — every replan must be traceable

## Dispatch-Ready Review

`/ordvik:planning dispatch-ready`

1. Scan backlog for P0/P1 items marked `task-ready: no`
2. For each: can it be decomposed into tasks now?
3. If yes: offer to run `/ordvik:dispatch create` for it
4. If no: note what's blocking task-readiness

## Daily Digest

`/ordvik:planning digest [<date>]`

One message a day for the operator, so the standing roles report in one place
rather than each on its own. The date defaults to today in the operator's
time zone; the digest covers the day before it, from the end of the previous
digest.

1. Fetch the planning repository with full history (unshallow it if it is
   shallow). For each project, read the role reports since the last digest,
   `<planning>/status/<date>-<role>.md` (one per role per day), the open
   asks (`ordvik planning asks --json --project <p>`), the stall register
   (`<planning>/stalled.md`), open PRs and blocked threads.
   Refresh the register (Step 2c's rules): add what newly qualifies, remove
   what cleared.
2. Write `<planning>/status/<date>-digest.md` per project that has anything,
   in this order, leaving out empty sections:

   ```markdown
   # Digest: <date>

   ## Needs you
   <asks by kind (decide, review, approve, do), oldest first, each with its recommendation and link>

   ## Merged by agents
   <each PR: level, who merged it and why; path-rule highs called out>

   ## Closed by the gardener
   ## Left for you to confirm
   ## Needs a thread
   <PRs whose thread ended with findings open, and missing-test items filed>

   ## Shipped
   ## Moving and stalled
   <one line per stream, from the latest planning pass and the day's merges;
   then the stall register's blockers, the one that clears most first, with
   what was added to or cleared from it today>

   ## Invariants
   <anything a role found broken, and whether it was fixed>
   ```

3. `ordvik planning check`, commit the digest and the register
   (`digest: <project> <date>`) and push.
4. Post the operator a short version: the **Needs you** count and the top
   three, what agents merged (a count and anything reverted or path-rule),
   and anything broken. Link the digest file. A day where nothing needs the
   operator and nothing merged gets one line.
5. Ask whoever starts threads (the coordinator) to start one for each PR
   under **Needs a thread**.

A role with nothing to report adds nothing.
