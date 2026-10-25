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
