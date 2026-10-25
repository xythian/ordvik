# Dispatch Conventions — Default

**Where planning lives.** `<planning>` below is the project's planning directory: `projects/<project>/` in the planning repository when `.ordvik.yml` names one (`planning: {repo: <owner>/<planning-repo>, project: <project>}`; cloned beside the code checkout as `planning/`, or at `$ORDVIK_PLANNING`), else `docs/planning/` in the repository. In the planning repository the backlog is one file per item in `backlog/` (ordered by `priority`, then `rank`) and threads are files in `threads/`, where `docs/planning/` has `backlog.md` and `threads.md`; status reports are in `status/` (else `docs/status/`); dispatch docs carry `id:`, `thread:` and `repo:` (else the id is `thread:`); plans list `dispatches:` (else `threads:`); and a change is a commit straight to its `main` (fetch, change, `scripts/plan check`, commit, push), not a PR. `scripts/plan` and `scripts/asks` read either layout (in the planning repository they run `ordvik planning`).

Plugin default conventions. Projects customize by creating `<planning>/conventions.md` which takes precedence over this file.

## Status Lifecycle

Dispatch documents track their lifecycle via a `status` frontmatter field:

```
active  →  done
        →  superseded
```

- **`active`** — Work is in progress or pending. This is the default; the field can be omitted and missing `status` is treated as `active`.
- **`done`** — All tasks completed and completion criteria verified. Set via the `close` operation in a last commit of the thread's own PR, so merging that PR is what makes the thread done on the default branch.
- **`superseded`** — Replaced by a newer dispatch doc. Set via `close --superseded-by <ref>`.

When superseding a dispatch doc, two fields are added:

```yaml
status: superseded
superseded_by: "20261023-new-slug"
```

And a blockquote is inserted immediately after the title heading:

```markdown
# Thread <id>: Original Title

> **Superseded** by [20261023-new-slug](./20261023-new-slug.md). This document is retained for historical context.
```

## Task Completion Markers

Tasks are marked complete by appending ` — DONE` to the task heading:

```markdown
## Task 1: Implement the widget — DONE
```

Rules:
- Use an em dash (`—`), not a hyphen or en dash
- No emoji — `— DONE` is the standard marker
- The marker is appended to the existing heading text, not placed on a new line
- Use the `mark-done` operation to apply the marker consistently

## Task Type Markers

Task headings can include a parenthetical type marker after the task number:

| Marker | Meaning |
|--------|---------|
| *(none)* | Standard implementation task |
| `(Human Gate)` | Requires human execution |
| `(Audit)` | Verification task; benefits from fresh agent context |
| `(Contract)` | Defines immutable tests/criteria for subsequent tasks |

Markers are part of the heading text and survive the `— DONE` append:

```markdown
## Task 5 (Audit): Verify data pipeline end-to-end — DONE
```

The `human_gates` frontmatter field lists Human Gate tasks. Audit and Contract tasks do not have a corresponding frontmatter field — they are identified by heading markers only.

## Plans, Not Waves

Waves (numbered batches launched together) are retired. Work that spans
more than one round of threads gets a plan in `<planning>/plans/` (see
`plans.md` beside this file): the plan owns ordering, a thread launches as
soon as what it waits on has merged, and `scripts/plan status` groups
threads into stages computed from those waits.

- **No `wave` field and no wave prefix** on new dispatch docs. Filename
  pattern: `YYYYMMDD-slug.md` (e.g., `20261023-add-auth.md`), and the
  name is the thread id. Older `TTT-slug.md` and `WWW-TTT-slug.md`
  docs keep their names.
- **Threads in a plan** carry `plan: <slug>` and no `depends_on`.
- **Standalone threads** (no plan) are unchanged: no `plan`, optional
  `depends_on`.

## Retro Format

Retros are written when a plan closes, in the plan's own **Retro** section,
using the template in the planning skill (`/ordvik:planning plan-close`).
A conductor may also write a short retro at an operator gate that ends a
phase. A large standalone thread (not in a plan, about five or more tasks,
or a week or more of work) gets a short retro too, in a `## Retro` section
of its dispatch doc, written in the closing commit. Small threads skip it.
A fix that took several failed attempts gets a TIL instead (or as well),
wherever the repo keeps them.

## Close-Out Checklist

### Thread-Level Close-Out

Before closing a dispatch thread (`/ordvik:dispatch close <file>`):

- [ ] All tasks marked `— DONE`
- [ ] Closing in the thread's own PR, as its last commit (not a follow-up PR)
- [ ] `status` set to `done` (or `superseded` with `superseded_by`)
- [ ] All completion criteria checked off
- [ ] Backlog updated — completed items marked done, follow-on items added if needed
- [ ] Pre-existing issues captured for gardening
- [ ] Audit tasks (if any) were executed and passed
- [ ] Contract tests (if any) still pass without modification to contract files

### Plan-Level Close-Out

Before closing a plan (`/ordvik:planning plan-close <slug>`):

- [ ] `scripts/plan status <slug>` shows every thread `done`, `superseded` or `dropped`
- [ ] Retro written in the plan
- [ ] Plan set to `status: done` and moved to `<planning>/plans/archive/`
- [ ] Backlog updated with follow-on items from retro
- [ ] Thread registry updated if thread statuses changed

## Archive Convention

Completed dispatch docs are archived to preserve the dispatch folder as a clean work queue.

**Archive location**: `<planning>/dispatch/archive/`

**Rules**:
- Files keep their original names when archived
- `done` and `superseded` threads are archived immediately, preferably in the thread's own PR (`scripts/plan` finds archived docs by their `thread:` field, so plans need no edit)
- The `list` operation excludes `archive/` by default — use `--archived` to include them

## Folder as Work Queue

The `<planning>/dispatch/` directory should contain only **active work**:

- **Active dispatch docs** — threads with work in progress (`status: active` or missing)
- **Pending dispatch docs** — threads written but not yet launched (in a plan, `blocked` or `ready`)

Everything else belongs in `archive/`. This convention makes the dispatch folder a scannable work queue — if it's in the folder, it needs attention.
