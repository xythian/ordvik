---
name: dispatch
description: Create or update dispatch documents that define agent-executable work with tasks, deliverables, and validation. Use when the user asks to write a dispatch doc, create a thread, plan agent work, or break down a backlog item into tasks.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git config *)
  - Task
  - AskUserQuestion
---

# Dispatch Document Skill

**Where planning lives.** `<planning>` below is the project's planning directory: `projects/<project>/` in the planning repository when `.ordvik.yml` names one (`planning: {repo: <owner>/<planning-repo>, project: <project>}`; cloned beside the code checkout as `planning/`, or at `$ORDVIK_PLANNING`), else `docs/planning/` in the repository. In the planning repository the backlog is one file per item in `backlog/` (ordered by `priority`, then `rank`) and threads are files in `threads/`, where `docs/planning/` has `backlog.md` and `threads.md`; status reports are in `status/` (else `docs/status/`); dispatch docs carry `id:`, `thread:` and `repo:` (else the id is `thread:`); plans list `dispatches:` (else `threads:`); and a change is a commit straight to its `main` (fetch, change, `scripts/plan check`, commit, push), not a PR. `scripts/plan` and `scripts/asks` read either layout (in the planning repository they run `ordvik planning`).

Creates and manages dispatch documents — structured work definitions that can be consumed by launch scripts to spin up agent worktrees automatically.

This skill is part of the **planning layer** — it activates when `<planning>/` exists. It reads `<planning>/threads.md` and `<planning>/backlog.md` for context when available, but gracefully degrades if they don't exist.

## Convention File Resolution

For dispatch-specific conventions, resolve in this order:

1. **Project-local**: `<planning>/conventions.md`
2. **Plugin default**: `${CLAUDE_SKILL_DIR}/references/conventions.md`

The dispatch format specification (YAML frontmatter, task structure, human gates) is canonical in this skill — it's part of the planning model, not project-specific configuration.

## Scope Awareness

Dispatch docs operate within the repo's scope model. Read `.ordvik.yml` to determine the repo model (single-scope vs. multi-scope).

In **multi-scope repos**, dispatch docs target one or more scopes. The `scope` field in frontmatter indicates which project the work targets. This determines:
- Which build/test scripts or project commands to reference in validation
- Which directories the agent should focus on
- Which RFC and design doc directories are relevant for context
- Which `paths` from `.ordvik.yml` define the scope's source boundaries

In **single-scope repos**, the `scope` field is omitted — all work targets the single codebase.

## What is a dispatch document?

A dispatch document defines a unit of parallelizable work (a "thread") with:
- Machine-parseable YAML frontmatter (consumed by launch scripts)
- A clear goal and context section
- Numbered tasks with scope, deliverables, and validation commands
- Completion criteria

Dispatch docs live in `<planning>/dispatch/` and are the bridge between the backlog (what to do) and agent execution (doing it).

## Parsing Invocation

The user invokes this skill as `/ordvik:dispatch <operation> [args...]`.

| Operation | Arguments |
|-----------|-----------|
| `create`  | `<slug> [--plan <slug>] [--source <ref>]` |
| `list`    | `[--plan <slug>] [--archived] [--status <s>]` |
| `review`  | `<file>` |
| `close`   | `<file> [--superseded-by <ref>]` |
| `mark-done` | `<file> <task-number>` |
| `plan` | `new <slug>` \| `status <slug>` \| `conduct <slug>` \| `amend <slug>` |

If invoked with no arguments, show help. If the user asks conversationally to "write a dispatch doc" or "create tasks for X", treat as `create`.

---

## File Format

### Filename

Pattern: `YYYYMMDD-<slug>.md` where:
- `YYYYMMDD` = the day the doc is written (e.g., `20261023`)
- `<slug>` = kebab-case human-readable description

The filename without `.md` is the thread id, and the frontmatter `thread:`
repeats it. Named ids need no reservation: two threads only collide when
they start the same day with the same slug, which is a real collision.
`scripts/plan check` (no plan argument) fails when two active docs share
an id.

Older docs are numbered (`015-<slug>.md`, thread `"015"`) and keep their
numbers; both kinds of id work everywhere. Some carry a `WWW-` wave
prefix (`006-015-<slug>.md`); waves are retired, so new docs have none.

### YAML Frontmatter (required)

The frontmatter is **machine-parsed** by launch scripts. The `branch` field is required — without it, the doc cannot be launched.

```yaml
---
thread: "20261023-widget-api"
branch: thread/widget-api
executor: agent
scope: search                    # multi-scope repos: which project this targets
source: RFC search/0069
status: active                 # active (default) | done | superseded
depends_on: []
human_gates:
  - "Task 7: Deploy and verify one service end-to-end"
---
```

Field reference:

| Field | Required | Description |
|-------|----------|-------------|
| `thread` | yes | The thread id: the filename without `.md` for new docs (`20261023-widget-api`); a number for older ones |
| `wave` | no | Retired: older docs only. Do not add it; plans order threads |
| `branch` | **yes** | Git branch name — launch scripts create a worktree with this branch |
| `executor` | yes | `agent`, `human`, or `mixed` — launch scripts skip `human` threads |
| `scope` | no | Target scope name from `.ordvik.yml` (multi-scope repos only). Omit for single-scope repos or root-scope work. |
| `source` | yes | Where this work comes from. Use scope-prefixed RFC refs in multi-scope repos (e.g., `RFC search/0069`). |
| `status` | no | Lifecycle state: `active` (default, can be omitted), `done`, or `superseded`. See conventions doc for lifecycle rules. |
| `plan` | no | Slug of the dispatch plan this thread belongs to (`<planning>/plans/<slug>.md`) |
| `depends_on` | no | Thread ids that must complete first. Leave it out when `plan` is set: the plan owns ordering |
| `human_gates` | no | List of task descriptions that require human execution |
| `platform` | no | `darwin`, `linux`, or `any` (default: `any`) — platform constraint |
| `repo` | no | Cross-repo dispatch target (see below) |
| `prs` | no | Planning repository only: the PRs that carry the work, each `{pr: "owner/name#n", tasks: [n, ...]}` or `{pr: ..., done: true}`. Written by `ordvik planning link`; see Linking PRs below |

### Linking PRs

In the planning repository, a thread links each PR it opens to its dispatch doc, and done is read from those PRs merging: no planning edit after the merge, and nothing about planning in the code repository.

```bash
ordvik planning link <project>/<id> --pr <owner/name#n> --task 1 --task 2   # this PR completes tasks 1 and 2
ordvik planning link <project>/<id> --pr <owner/name#n> --done              # this PR completes the dispatch
```

Run it from the planning checkout right after opening the PR; it commits to the planning repository's `main` and pushes. A task is done once a PR that lists it merges; the dispatch is done once a PR linked with `--done` merges and every human gate is marked. Gates stay open until the operator's answer is recorded.

Commits, branch names, PR titles and bodies, and code comments in the code repository never name the dispatch, plan or planning repository: no `Dispatch:` trailer, no planning links. In a repository its `project.yml` lists as `{repo: ..., public: true}` this is enforced (`scripts/check-attribution --public`), and there the branch is a plain slug rather than the dispatch id.

### Cross-Repo Dispatch

Work sometimes targets a different repo than the one hosting the planning system. The **planning hub** is the repo that holds the backlog, thread registry, and dispatch docs. The **target repo** is where the implementation happens.

Two topologies use this:

- **Hybrid** — A monorepo acts as both planning hub and implementation repo for most work, but dispatches some threads to satellite repos. Most dispatch docs omit `repo` (same-repo); some include it.
- **Pure planning hub** — A repo that contains only planning artifacts (backlog, threads, dispatch docs, RFCs) with no implementation code. Every dispatch doc targets an external repo via `repo`. This suits organizations that keep each project in its own repo but want centralized planning.

**The `repo` field** declares a cross-repo target:

```yaml
---
thread: "020"
branch: feat/initial-structure
executor: agent
source: RFC 0070
repo:
  path: ../ordvik                              # Relative to planning hub
  url: git@github.com:xythian/ordvik.git       # Clone here if path doesn't exist
  add_dirs:
    - ../monorepo                              # Reference access to planning hub
---
```

| `repo` sub-field | Description |
|------------------|-------------|
| `path` | Relative path from the planning hub to the target repo |
| `url` | Git clone URL — used if `path` doesn't exist yet |
| `add_dirs` | Directories the agent needs reference access to (typically the planning hub itself) |

**How it works today (v0.1):** Manual launch. The human ensures the target repo exists, then launches the agent there with a reference back to the dispatch doc:

```bash
cd ../ordvik
claude "Read and execute the dispatch document at \
  ../monorepo/<planning>/dispatch/020-widget-api.md"
```

The `repo` field documents the relationship even when launch is manual — it tells the human (and future tooling) which repo to target, where to clone it from, and what reference access is needed.

**Future automation:** Launch scripts can read `repo`, ensure the target exists (clone if needed), create a branch there, and launch the agent with `--add-dir` for each entry in `add_dirs`. Dispatch docs without `repo` work exactly as before — same-repo dispatch is the default.

**Conventions for cross-repo dispatch docs:**

- The dispatch doc lives in the **planning hub**, not the target repo
- The Context section should note that the agent is working in a different repo and list which paths from the planning hub it may need to read
- `source` references use the planning hub's RFC numbering
- The `branch` is created in the **target repo**, not the planning hub

### Document Body

```markdown
# Thread <id>: Title

**Goal**: One-sentence description of what this thread accomplishes.

## Context

Background information the agent needs. Include:
- What problem this solves and why
- Key files to read (with paths)
- Critical design constraints
- Links to relevant RFCs or design docs

---

## Task N: Title

**Scope**: What this task accomplishes (one paragraph).

**Deliverables**:
- Specific files to create or modify
- Specific behaviors to implement
- Be concrete — list file paths, function names, types

**Automated validation**:
```bash
./scripts/test.sh full
./scripts/build.sh relevant-binary
```

**Human validation**: Description of what a human should check after deployment (optional).

---

## Task N (Human Gate): Title

> **Human gate**: Description of why this requires human execution.

Steps the human needs to perform, with commands and checklists.

---

## Task N (Contract): Title

**Scope**: What tests/acceptance criteria this task defines.

**Deliverables**:
- Test files to create (these become immutable for subsequent tasks)
- Acceptance criteria document if applicable

**Automated validation**:
```bash
# Tests should compile but fail (no implementation yet) — that's expected
./scripts/test.sh full
```

---

## Task N: Title

**Contract**: Task M — tests in `path/to/tests` are immutable.
Do not modify test files. Make them pass.

**Scope**: What this task accomplishes (one paragraph).

**Deliverables**:
- Specific files to create or modify (NOT including contract test files)

**Automated validation**:
```bash
./scripts/test.sh full
```

---

## Task N (Audit): Title

**Scope**: What this task verifies — stated from scratch, not referencing
earlier task outcomes.

**Expected state**:
- File `path/to/file.go` exists and exports `FunctionName(args) returnType`
- Running `curl localhost:8080/health` returns 200
- Table `widgets` has columns: id, name, created_at

**Automated validation**:
```bash
./scripts/test.sh full
# Additional targeted verification commands
```

---

## Testing

Shared testing strategy across all tasks (unit, integration, regression).

---

## Constraints

- What the agent must NOT change
- Scope boundaries
- Behavioral invariants to preserve

## Pre-PR Checklist

- [ ] Full test suite passes (`./scripts/test.sh full`)
- [ ] Relevant integration tests pass (see `./scripts/test.sh list`)
- [ ] Full build succeeds (`./scripts/build.sh`)
- [ ] All completion criteria verified
- [ ] No stale references to removed/renamed code

## Pre-existing issues encountered

<!-- Note any broken tests, warnings, or issues you found
     that predate this thread's work. These will be picked up
     by the next gardening run. -->

## Completion Criteria

- [ ] Measurable acceptance criterion 1
- [ ] Measurable acceptance criterion 2
- [ ] All unit tests pass
- [ ] PR ready for review
```

---

## Operations

### `create <slug> [--plan <slug>] [--source <ref>]`

1. **Check planning structure** — If `<planning>/dispatch/` doesn't exist, create it. Note whether `<planning>/threads.md` and `<planning>/backlog.md` exist (used for context but not required).

2. **Determine repo model** — Read `.ordvik.yml` if it exists. In multi-scope repos, determine which scope this dispatch targets (from `--source` RFC prefix, from conversation context, or ask the user). Read the scope's config for `paths`, `language`, and `designs` directory.

3. **Gather context** — Use `AskUserQuestion` to understand:
   - What work needs to be done? (or read from the referenced backlog item/RFC)
   - What thread does this belong to? (check `<planning>/threads.md` if it exists)
   - Is this part of a plan? (check `<planning>/plans/`; if so, add `plan:` and its entry in the plan, not `depends_on`)
   - What is the source? (RFC, backlog item, ad-hoc). Use scope-prefixed RFC references in multi-scope repos.
   - Which scope does this target? (multi-scope repos only)
   - Does the work happen in a different repo? If so, gather `repo` fields (path, url, add_dirs).
   - Who executes? (agent, human, mixed)
   - Are there platform constraints? (darwin, linux, any)

4. **Research** — Before writing tasks, understand the codebase:
   - Read the source RFC or design doc (use the scope's `designs` directory for design docs)
   - Read the key files that will be modified (use the scope's `paths` to focus)
   - Understand existing patterns and conventions
   - Check for dependencies on other threads

5. **Decompose into tasks** — Break the work into sequential tasks. Each task should:
   - Be completable in isolation (within the thread)
   - Have concrete, verifiable deliverables
   - Include automated validation commands
   - Take no more than ~1 hour of agent time
   - Build on previous tasks in the sequence
   - Limit threads to 5–7 implementation tasks. If decomposition yields more, split into multiple threads with an explicit dependency chain. Long threads accumulate context that biases the agent toward confirming its own earlier work.

6. **Prompt for verification completeness** — For each task, consider:
   - "How would you know this task is *actually* done, not just 'tests pass'?"
   - "What could go wrong that unit tests wouldn't catch?"
   - "Is there an end-to-end check that would validate the full data flow?"
   - "Is the accumulated context going to bias late-thread verification? Should this have an audit task?"
   - Flag tasks that have only unit-level validation for code that touches multiple subsystems.

   **Decision rules:**
   - If the thread has 4+ implementation tasks, add an `(Audit)` task as the final task before the Pre-PR Checklist.
   - If the work has clearly specifiable acceptance criteria (APIs, data flows, behavioral contracts), consider leading with a `(Contract)` task that writes tests before implementation begins.

7. **Identify human gates** — The operator is asked only for judgement or a decision (see `references/process.md` beside the plugin's skills, "The operator's time"). Mark a task as a human gate only when it is one of:
   - **Approve**: a production step built from existing commands. The thread shows the dry run, the operator approves it, and the thread runs `--commit` on a venue that holds the authority. Word it as an approval and name the command.
   - **Decide**: a judgement call or a choice between options.
   - **Do**: something that needs the operator's own hands, devices or accounts and no command covers. Each one is a candidate to automate.

   A list of commands for the operator to type is not a gate; it is a missing command.

8. **Write the document** — Create the file at `<planning>/dispatch/YYYYMMDD-<slug>.md` (today's date) following the format above. In multi-scope repos, include the `scope` field in frontmatter. Always include the Pre-PR Checklist and Pre-existing issues sections.

9. **Report** — Show the user: file path, thread id, plan (if any), scope (if multi-scope), task count, and any human gates.

### `list [--plan <slug>] [--archived] [--status <s>]`

1. Glob `<planning>/dispatch/*.md` (exclude human-week docs). By default, exclude `archive/` subdirectory.
2. If `--archived` is given, also glob `<planning>/dispatch/archive/*.md` and include those results.
3. Parse YAML frontmatter from each file. Treat missing `status` as `active`.
4. If `--plan` is given, filter to docs with that `plan`.
5. If `--status` is given, filter to that status only (e.g., `--status done`).
6. Output a table: **File**, **Thread**, **Plan**, **Status**, **Executor**, **Source**, **Branch**.

### `close <file> [--superseded-by <ref>]`

Closes a dispatch thread by updating its status and archiving. Read `${CLAUDE_SKILL_DIR}/references/conventions.md` for full lifecycle rules.

1. **Read** the dispatch document.

2. **If `--superseded-by` is given** (supersession close):
   - Set `status: superseded` in frontmatter
   - Add `superseded_by: "<ref>"` field to frontmatter
   - Insert a blockquote after the title heading: `> **Superseded** by [<ref>](./<ref>.md). This document is retained for historical context.`
   - Archive immediately — move to `<planning>/dispatch/archive/`
   - Report what was done

3. **If closing as done** (normal close):
   - Validate all tasks have the `— DONE` marker. If any are missing, report which tasks are not marked done and ask the user to confirm or mark them.
   - Validate all completion criteria checkboxes are checked (`[x]`). Report any unchecked items.
   - Set `status: done` in frontmatter
   - Archive immediately to `<planning>/dispatch/archive/` (in the thread's own PR when it has one; in the planning repository, a commit there whenever the files want tidying, since a `--done` PR link already makes it done)
   - If the doc has `plan:`, run `scripts/plan status <slug>`. If every thread is finished, prompt the user to run `/ordvik:planning plan-close <slug>`; otherwise tell the plan's conductor what this unblocks.
   - Ask the user about backlog follow-ons — are there new items to add from this work?

### `mark-done <file> <task-number>`

Convenience operation to standardize the task completion marker.

1. Read the dispatch document.
2. Find the task heading matching the given number (e.g., `## Task 3: ...`).
3. If the heading already has `— DONE`, report that it's already marked and do nothing.
4. Append ` — DONE` to the heading text.
5. Write the updated file.

### `plan new|status|conduct|amend <slug>`

Dispatch plans: work that spans more than one round of threads gets a plan file in
`<planning>/plans/<slug>.md` listing its threads, what each waits on (a
thread, one task of a thread, or an operator gate), and a log of changes.
One agent, the plan's **conductor**, keeps it moving. Read
`${CLAUDE_SKILL_DIR}/references/plans.md` first: it defines the format, the
thread states, the conductor's loop and how plans change.

The status tool is `scripts/plan` (installed by `/ordvik:init` from
`${CLAUDE_PLUGIN_ROOT}/scripts/plan`). If the repo lacks it, run it from the
plugin: `python3 ${CLAUDE_PLUGIN_ROOT}/scripts/plan ...`.

- **`new`**: write `<planning>/plans/<slug>.md` (frontmatter `plan`,
  `title`, `status`, `source`, `conductor`, `threads`, `gates`; body with
  Decisions, Threads, Not in this plan, Log). Write dispatch docs for the
  threads that can start first with `create`, adding `plan: <slug>` and no `depends_on`.
  Later threads may stay sketches (`title` and `after`, no doc). Ordering
  that hangs on part of a thread is a task dependency (`"101:2"`), not a
  whole-thread one. Run `scripts/plan check <slug>` before the PR.
- **`status`**: `scripts/plan status <slug> --fetch`, then say what can
  launch, what needs a doc, and which gates to ask.
- **`conduct`**: one pass of the conductor's loop in the reference:
  launch `ready` threads (not ones already launched), write docs for
  `write-doc` sketches, ask pending gates, report only what changed.
- **`amend`**: change `threads`, `after`, `gates` or `dropped`, add a dated
  Log line (what, why, who asked), tell launched threads the change affects,
  `scripts/plan check`, docs PR.

### `review <file>`

1. Read the dispatch document.
2. Check against this checklist:
   - [ ] YAML frontmatter present with required fields (`thread`, `branch`, `executor`, `source`)
   - [ ] `branch` field is set (required for launch scripts)
   - [ ] `status` field, if present, is one of: `active`, `done`, `superseded`
   - [ ] If `status: superseded`: `superseded_by` field is present and the superseding doc exists
   - [ ] If `status: done`: all task headings have `— DONE` marker
   - [ ] In multi-scope repos: `scope` field is present and matches a declared scope in `.ordvik.yml`
   - [ ] `source` uses scope-prefixed RFC references when applicable (e.g., `RFC search/0069`)
   - [ ] Goal statement is clear and specific
   - [ ] Context section includes key file paths (within the target scope's `paths`)
   - [ ] Each task has: Scope, Deliverables, Automated validation
   - [ ] Automated validation uses project build/test scripts (not bare `go test`, `cargo test`, etc.)
   - [ ] Human gates are marked in both the task heading and `human_gates` frontmatter
   - [ ] Pre-PR Checklist is present and includes full test suite (not just fast tier)
   - [ ] Constraints section sets clear boundaries
   - [ ] Completion criteria are measurable
   - [ ] Verification tiers are appropriate — tasks touching multiple subsystems have more than unit-level validation
   - [ ] Threads with 4+ implementation tasks include at least one `(Audit)` task
   - [ ] Audit task bodies are self-contained — no references to "Task N created/did X"
   - [ ] If `(Contract)` tasks exist: bound implementation tasks include a **Contract** field
   - [ ] If `(Contract)` tasks exist: contract deliverable files are not listed as modifiable in implementation task deliverables
   - [ ] If `plan` is present: the plan lists this thread, the doc has no `depends_on`, and `scripts/plan check <slug>` passes
   - [ ] If `repo` is present: `path` and `url` are set, `add_dirs` includes the planning hub, Context section notes the cross-repo setup
3. Report findings and suggest fixes.

---

## Task Writing Guidelines

Good tasks are **specific enough that an agent can execute without asking clarifying questions**.

**Do**:
- List exact file paths to create or modify
- Include the validation commands verbatim (copy-pasteable)
- Reference specific types, functions, or interfaces by name
- Set explicit scope boundaries ("do NOT modify X")
- Use project build/test scripts, not bare language-specific commands
- Always specify the full test suite (`./scripts/test.sh full`), never a subset or fast tier — abbreviated test runs hide regressions

**Don't**:
- Write vague deliverables ("improve the code", "add tests")
- Assume the agent knows which files to modify
- Skip automated validation — every task needs a way to verify it worked
- Use abbreviated or fast test tiers for task validation — those are for interactive development, not declaring work complete
- Combine unrelated changes in one task
- Include tasks that depend on external systems the agent can't access (mark those as human gates)

## Verification Tiers

Each task's validation section should distinguish when appropriate:

- **Automated (agent-run):** Commands the agent runs locally — unit tests, builds, linters, dry-runs
- **Integration (agent-run if infra available):** Commands that require infrastructure — VM-based tests, database tests, network tests. Specify what infrastructure is needed.
- **Operational (human-run):** Checks that require deployment or access the agent doesn't have — dashboard verification, production log checks, user-facing behavior

The pre-PR checklist always requires the full test suite plus relevant integration tests — never an abbreviated or fast tier alone. The fast tier is for iterative development feedback; the full suite is for declaring work complete. Every automated validation command in every task must run the full test suite, not a subset.

## Task Patterns

Beyond standard implementation tasks and human gates, dispatch docs support two additional task patterns that address context poisoning — the tendency for agents to confirm their own earlier work rather than verify it independently.

### Audit Tasks

An audit task verifies prior work with **fresh context**. It is designed to be executed by a separate agent session that has NOT accumulated the executing agent's "I did this" context.

Mark audit tasks in the heading:

    ## Task 5 (Audit): Verify data pipeline end-to-end

**Rules:**

- The task body must be **self-contained**: re-state what should exist, what behavior to verify, and which files to inspect. Do not write "verify Task 2's deliverables" — instead list the specific files, functions, and behaviors.
- Include an **Expected state** field listing concrete, checkable assertions about the state of the codebase.
- Automated validation commands are mandatory. The auditor runs them from scratch.
- Audit tasks must NOT reference earlier task outcomes ("Task 2 created X"). State the expected state of the world as fact: "The file X should exist and export function Y with signature Z."
- Place audit tasks after the implementation tasks they verify, typically as the last task before the Pre-PR Checklist.
- For threads with 4+ implementation tasks, include at least one audit task.

Launch scripts and human operators should treat `(Audit)` tasks the same as `(Human Gate)` in one respect: they benefit from a fresh agent session. Future launch script versions may enforce this automatically.

### Validation Contracts

A validation contract is a task that writes tests or acceptance criteria BEFORE the implementation tasks that must satisfy them. The tests become an immutable oracle — implementation must make them pass, not modify them.

Mark the contract task:

    ## Task 1 (Contract): Define acceptance tests for widget rendering

Mark implementation tasks that are bound by the contract:

    ## Task 2: Implement widget renderer

    **Contract**: Task 1 — tests in `src/widget/widget_test.go` are immutable.
    Do not modify test files. Make them pass.

**Rules:**

- The contract task writes test files and records their paths in its Deliverables.
- Contract tests should run the full test suite, not a subset. Tests must compile but are expected to fail initially (no implementation yet).
- Implementation tasks list the contract they are bound by in a **Contract** field (after Scope, before Deliverables).
- Agents executing implementation tasks must NOT modify files listed in the contract's Deliverables unless the contract explicitly permits specific changes (e.g., "test helpers in `testutil/` may be extended but not altered").
- If an implementation task cannot pass a contract test, it must document the gap in the Pre-existing issues section rather than silently editing the test.
- Contract tests should include both unit-level and behavioral/integration-level checks to prevent narrow implementations that technically pass but miss intent.

### When to Use Each Pattern

| Signal | Pattern |
|--------|---------|
| Thread has 4+ implementation tasks | Add an `(Audit)` task |
| Work has clearly specifiable acceptance criteria (APIs, data flows, behavioral contracts) | Lead with a `(Contract)` task |
| Thread modifies multiple subsystems that interact | Both — contract for the interface, audit for integration |
| Simple 2–3 task thread with good automated tests | Neither — standard tasks are fine |

## Inline Markers

Dispatch docs support the same COMMENT/QUESTION marker convention used in RFC authoring. See the RFC authoring guide for the full specification.

During dispatch doc creation, the agent may place `> QUESTION: ...` markers (blockquoted) for decisions that need operator input (e.g., verification approach, scope boundaries, human gate placement). The user may add `COMMENT: ...` markers (plain text, no blockquote) during review. The user responds to QUESTIONs by either editing the text directly or adding a COMMENT. Process both the same way: address, update, remove.
