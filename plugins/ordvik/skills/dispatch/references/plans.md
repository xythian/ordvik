# Dispatch plans — reference

**Where planning lives.** `<planning>` below is the project's planning directory: `projects/<project>/` in the planning repository when `.ordvik.yml` names one (`planning: {repo: <owner>/<planning-repo>, project: <project>}`; cloned beside the code checkout as `planning/`, or at `$ORDVIK_PLANNING`), else `docs/planning/` in the repository. In the planning repository the backlog is one file per item in `backlog/` (ordered by `priority`, then `rank`) and threads are files in `threads/`, where `docs/planning/` has `backlog.md` and `threads.md`; status reports are in `status/` (else `docs/status/`); dispatch docs carry `id:`, `thread:` and `repo:` (else the id is `thread:`); plans list `dispatches:` (else `threads:`); and a change is a commit straight to its `main` (fetch, change, `scripts/plan check`, commit, push), not a PR. `scripts/plan` and `scripts/asks` read either layout (in the planning repository they run `ordvik planning`).

A single dispatch doc launches well: `wt launch`, a remote agent host, or
an agent session (for example a Claude project thread) told to execute it. Work that spans
several rounds of threads needs more: one place that says what waits for
what, a way to see what can start now, someone who notices when it can,
and a way to change the plan as we learn without losing track of why.

This doc defines that: the **plan** (a file), the **status tool**
(`scripts/plan`), and the **conductor** (the agent that keeps the plan
moving). The dispatch doc format, its lifecycle and closing it in its own
PR are unchanged (see the dispatch skill and its conventions).

## Principles

1. **Git is the record.** The plan is a file on the default branch. What
   is done is read from the remote's default branch, not from anyone's memory: a thread is done
   when its dispatch doc says `status: done`, and a task is done when its
   heading ends in `— DONE`. When a dispatch closes in its own PR (the
   recommended convention), "merged" and "done on the default branch" are
   the same thing.
2. **Launch when ready; no waves.** Dependencies are per thread, and can
   name a single task. A thread starts as soon as what it needs has merged.
   Waves existed so a person (and a glob) could launch the right batch at
   once; `scripts/plan ready` does that now, so plans have no waves.
   `scripts/plan status` groups threads into **stages** computed from what
   they wait on (stage 1 waits on no other thread, stage 2 only on stage 1,
   and so on), which is the grouping waves gave, without a second label
   that can drift from the graph. Dispatch docs in a plan have no `wave:`
   and no `WWW-` filename prefix.
3. **Late binding.** Later threads can stay one-line sketches until their
   dependencies are nearly done. Their dispatch docs are written then, from
   what the earlier threads learned.
4. **One place to change.** Ordering lives only in the plan. Dispatch docs
   in a plan name the plan and leave `depends_on` out. The "what can start
   now" table is computed, never hand-kept.
5. **A plan has one conductor.** One agent owns watching it and acting on
   it, and the operator steers the plan by talking to that agent.

## The plan file

Plans live in `<planning>/plans/<slug>.md` and move to
`<planning>/plans/archive/` when done. The frontmatter is the machine
part; the body is for people.

```yaml
---
plan: bootstrap-drift            # the slug; dispatch docs name it in `plan:`
title: Bootstrap drift and IAM follow-through
status: active                   # draft | active | done | abandoned
source: bootstrap drift review (operator, 2026-10-22)
conductor: "Claude project thread: Bootstrap drift and IAM status"
threads:
  - {id: "100"}
  - {id: "101"}
  - {id: "104", after: ["100:1", "101:2"]}
  - {id: "106", after: ["100", "101", "102"]}
  - id: "107"                    # a sketch: no dispatch doc yet
    title: IAM service, RFC 0074 Phase 1
    after: ["105", "gate:iam-go"]
  - {id: "103", launched: "2026-10-22 project thread"}
  - {id: "108", dropped: "2026-10-24: folded into 104"}
gates:
  - {id: iam-go, ask: "Start RFC 0074 Phase 1 once principals are in the catalog?"}
  - {id: rfc-0095, ask: "Accept RFC 0095?", passed: 2026-10-25, note: "accepted with changes"}
---
```

Thread fields:

| Field | Meaning |
|---|---|
| `id` | The thread id (`20261023-add-auth`, or a number for older docs). Its dispatch doc is found by its `thread:` frontmatter, in `dispatch/` or `dispatch/archive/`, so archiving needs no plan edit. |
| `after` | What it waits for. `"100"`: thread 100 is done. `"100:2"`: thread 100's Task 2 is marked done on the default branch (or 100 is done). `"gate:<id>"`: the plan's gate has `passed:`. |
| `title` | Only for a sketch, which has no dispatch doc yet. |
| `launched` | Optional record that it was started and where, for launches git cannot see (a project thread works on its own branch, not the doc's). |
| `dropped` | Why it was taken out, with a date. Kept so the history reads. |

Gates are the operator's decisions the plan waits on: whether to start a
phase, accepting an RFC another thread needs, a production result that
decides what comes next. A gate passes when the conductor records `passed:`
with the date and the operator's answer. A human gate *inside* a thread
(Task 7 deploys to production) stays in its dispatch doc, as now; it
becomes a plan gate only when other threads wait on it, and then it is
usually better as a task dependency (`"101:7"`).

A task dependency only resolves when that task's `— DONE` reaches
the default branch, so a thread that others wait on lands those tasks in their own
PRs. `scripts/plan status` lists them under each thread ("Task 2 unblocks:
104, 105"), and the thread's brief says so.

The body, in this order:

- What the plan is for and where it came from.
- **Decisions**: the operator's answers that shaped it, dated.
- **Threads**: one row per thread with its scope, size and the shared
  files to watch, and a short paragraph on why the parallel ones can run
  together. No status or ordering columns; both are computed.
- **Not in this plan**: what was considered and left out.
- **Log**: one dated line per change to the plan (see Changing the plan).
- **Retro**, when done.

Dispatch docs in a plan add `plan: <slug>` to their frontmatter and leave
`wave:` and `depends_on:` out (the plan owns ordering; `scripts/plan
check` warns about either).

## The status tool

`scripts/plan` reads the plan from the working tree and every dispatch doc
from a ref (default `origin/HEAD`, else `origin/main` or `origin/master`;
`--fetch` fetches first). It needs git and PyYAML. `/ordvik:init` offers to
install it from `${CLAUDE_PLUGIN_ROOT}/scripts/plan`.

```bash
scripts/plan status bootstrap-drift --fetch       # the table, by stage, and what to do next
scripts/plan status bootstrap-drift --json        # the same, for an agent
scripts/plan ready bootstrap-drift                # paths of docs to launch now, one per line
scripts/plan check bootstrap-drift                # validate; exit 1 on errors
```

Each thread gets one state:

| State | Meaning | Conductor's action |
|---|---|---|
| `ready` | Doc on the default branch, everything in `after` holds, no sign it started | Launch it |
| `started` | Launched (`launched:`, the doc's branch on origin, or a task done) | Watch it |
| `blocked` | Doc exists, waiting on `after` | Nothing, or write ahead |
| `write-doc` | A sketch whose `after` all hold | Write its dispatch doc |
| `planned` | A sketch still waiting | Nothing yet |
| `human` | `executor: human`, ready | Tell the operator |
| `done`, `superseded`, `dropped` | Finished one way or another | Nothing |

`status` ends with "Launch now", "Write the dispatch doc" and "Gates to
ask the operator", and says when every thread is done. `check` catches a
plan that cannot work: an `after` naming a thread, task or gate that does
not exist, a cycle, a doc that names the plan but is not in it, and doc
frontmatter that is not valid YAML (which `wt launch` would also choke
on).

`ready` composes with the launchers:

```bash
scripts/wt launch $(scripts/plan ready bootstrap-drift --fetch) --claude
```

## The conductor

The conductor is one agent session that owns a plan until it closes: in a
Claude project, a project thread, normally the one where the plan was
written; elsewhere, a long-lived session the operator can return to. The
operator steers the plan by writing there. The plan's `conductor:` field names it so anyone can
find it.

### What wakes it

The conductor never polls in a loop. It wakes on:

1. **PR events.** When a thread's PR is up, the executing thread sends the
   link to the conductor, which subscribes to it. A merge wakes the
   conductor.
2. **Milestone messages.** The executing thread messages the conductor
   when a task others wait on merges, when it hits a gate or a blocker
   that affects other threads, and when it learns something that should
   change a later thread.
3. **A fallback check-in.** A scheduled wake (`send_later`, a trigger, or
   `/loop` in a terminal session) (a day at first, backing off
   while nothing changes) in case an event is missed. After a few quiet
   check-ins the conductor tells the operator what the plan is waiting on
   and stops scheduling until something changes.
4. **The operator**, writing in the conductor's thread.

### What it does on each wake

1. `git fetch origin` and `scripts/plan status <slug>`.
2. **Launch now**: launch each `ready` thread (below), and keep the list of
   what it launched where (its checklist) until the next plan edit records
   `launched:`. A `ready` thread the conductor already launched is not
   launched again.
3. **Write the dispatch doc**: for each `write-doc` sketch, write the doc
   with what the merged threads learned (their PRs, their docs' "Pre-existing
   issues", messages from them), add `plan:` to it, and open a docs PR. Do
   this as soon as the dependencies are close, so the doc is merged by the
   time they are.
4. **Gates**: ask the operator once per gate, with a decision card that
   says what it unblocks. Record the answer as `passed:` (or change the
   plan if the answer is no).
5. **Report** only when something happened the operator should know: a
   launch, a gate, a blocker, a finished stage. Status that needs nothing
   from them goes on the checklist.
6. When every thread is done: write the retro in the plan, set
   `status: done`, move it to `plans/archive/`, and update the backlog and
   `<planning>/threads.md` (if they exist).

### Launching

| Where | How |
|---|---|
| A Claude project thread | Ask the coordinator to start a thread per doc, with the brief below |
| A remote agent host | Whatever launches one dispatch there today, once per `ready` doc |
| A local worktree | `scripts/wt launch $(scripts/plan ready <slug>) --claude` |

The brief for an executing thread:

> Execute the dispatch doc `<path>` on the default branch. It is thread `<id>` of
> plan `<slug>` (`<planning>/plans/<slug>.md`); the conductor is
> `<conductor thread>`. Other threads wait on your `<Task N, ...>`: land
> each in its own PR, marked `— DONE`, and tell the conductor when it
> merges. Send the conductor your PR link when it is up, and tell it about
> anything you learn that changes later threads. Close the dispatch in your
> own PR.

The milestones come from `scripts/plan status --json` (`milestones` on
each thread).

## Changing the plan

Plans change as we learn; that is the point of late binding. Every change
goes through the conductor, so there is one writer:

1. The operator asks in the conductor's thread, or the conductor proposes
   a change after a thread reports something (a decision card when it
   changes the operator's goal).
2. The conductor edits the frontmatter (`threads`, `after`, `gates`,
   `dropped`) and appends a Log line: date, what changed, why, who asked.
3. Docs of threads not yet launched are edited freely. A launched thread is
   told about the change by message; if its scope changes materially, its
   doc is superseded (`close --superseded-by`) and a new thread replaces
   it in the plan.
4. `scripts/plan check` passes, then a docs PR. The plan is what's on
   the default branch; until the PR merges, the conductor acts on its branch's
   version and says so.

Recording `launched:` and `passed:` rides along with the next such edit;
it does not need its own PR.

## Why not more

- **No runtime state in git beyond the plan.** Who is running what lives
  with the conductor and in the PRs. Git holds what was decided and what
  merged.
- **No new scheduler.** The pieces exist: PR subscriptions, thread
  messages and scheduled wakes in Claude projects, `/loop` and tmux
  sessions elsewhere. The plan and `scripts/plan` give them a shared, checkable
  picture.
- **No GitHub API in the tool.** `scripts/plan` needs only git and
  PyYAML, so it runs the same in a cloud container, on a remote agent
  host and on a laptop.
