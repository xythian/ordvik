---
name: planning
description: Run a planning session — review threads, triage backlog, plan waves, and generate a planning report. Use when the user asks for a planning review, weekly planning, or wants to assess project status.
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

Runs a planning session: reviews all threads, triages the backlog, identifies ready work, and generates a planning report. This is the orchestration layer that connects threads, backlog, dispatch docs, and waves.

Requires the planning structure (`docs/planning/`). If it doesn't exist, suggest running `/ordvik:init` first.

## Parsing Invocation

The user invokes this skill as `/ordvik:planning [focus]`.

- Without arguments: full planning session (all threads, full backlog review)
- With a focus: targeted session (e.g., `/ordvik:planning wave-close 003`, `/ordvik:planning replan`)

| Focus | Description |
|-------|-------------|
| (none) | Full planning session |
| `wave-close <N>` | Close out a completed wave — update backlog, threads, write retro |
| `replan` | Replanning session — assess what changed and adjust |
| `dispatch-ready` | Review backlog for items ready to dispatch |

## Full Planning Session

### Step 1: Read Planning Artifacts

Read the authoritative sources:

1. `docs/planning/threads.md` — the thread registry
2. `docs/planning/backlog.md` — prioritized work items
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
5. Active dispatch docs: check `docs/planning/dispatch/` for in-progress work

**Stable/dormant/parked threads** — lighter review:
1. Recent commits: same check, expect "None" — flag unexpected activity
2. Brief note on what would trigger reactivation

Use Task/Explore agents to parallelize research across many threads. Thoroughness matters more than speed.

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
```

Key principles:
- **Every registry thread gets a section** — no silent drops
- **Be specific about next steps** — actionable, not vague
- **"Recent activity: None"** makes stale threads visible
- **"Consciously Deferred"** forces explicit acknowledgment

### Step 5: Route the Report

Use the same output routing as `/ordvik:status`, with suffix `planning`:
- Local: `docs/status/YYYY-MM-DD-planning.md`
- Obsidian: `projects/{project}/YYYY-MM-DD-{project}-planning`

### Step 6: Update Planning Documents

After generating the report, update if needed:

**`docs/planning/threads.md`** — if:
- New threads were discovered
- A thread's status changed
- The user provided corrections

**`docs/planning/backlog.md`** — if:
- Items were completed
- Priorities need adjusting
- New items emerged
- Dependencies changed

Always tell the user what document changes you made.

---

## Wave Close-Out

`/ordvik:planning wave-close <N>`

1. Read all dispatch docs for wave N
2. **Close each thread** — For each dispatch doc in the wave, run the dispatch close-out process:
   - Verify all tasks are marked `— DONE`
   - Verify completion criteria are checked
   - Set `status: done` in frontmatter (or `superseded` if applicable)
   - See `/ordvik:dispatch close` for the full thread-level checklist
3. Check completion status of each thread (merged PRs, branch state)
4. Update backlog — mark completed items with date, note follow-ups
5. Update thread registry if thread statuses changed
6. Write wave checkpoint: `docs/planning/dispatch/wave-NNN-retro.md`
7. **Archive all wave docs** — Move all wave dispatch docs and the retro to `docs/planning/dispatch/archive/`. Files keep their original names. This clears the dispatch folder of completed work.

Note: Dispatch conventions for status lifecycle and archive rules are defined in the dispatch skill's conventions doc (`references/conventions.md` within the dispatch skill). Wave close-out orchestrates thread close-outs as a sub-step — thread-level format concerns (status fields, DONE markers, supersession) are owned by the dispatch skill.

Retro format:

```markdown
# Wave NNN Retro

**Date**: YYYY-MM-DD
**Threads**: [list]

## Completed
- [Thread] — [summary of what shipped]

## Still Open
- [Thread] — [what remains, why]

## Scope Changes
- [What changed from original dispatch and why]

## Unblocked
- [What's now ready for the next wave]

## Observations
- What worked well
- What didn't
- Process improvements for next wave
```

## Replanning

`/ordvik:planning replan`

1. Assess current state — what changed since the last plan?
2. Review in-progress dispatch docs for scope changes
3. **Check for stale dispatch status** — Identify dispatch docs with `status: active` (or missing status) where work is clearly done or superseded. Flag these for close-out.
4. Identify blocked or delayed threads
5. Update dispatch docs (add tasks, mark descoped items with reason)
6. Update backlog priorities
7. Adjust wave boundaries if needed
8. Record the decision — every replan must be traceable

## Dispatch-Ready Review

`/ordvik:planning dispatch-ready`

1. Scan backlog for P0/P1 items marked `task-ready: no`
2. For each: can it be decomposed into tasks now?
3. If yes: offer to run `/ordvik:dispatch create` for it
4. If no: note what's blocking task-readiness
