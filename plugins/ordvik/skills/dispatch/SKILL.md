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

Creates and manages dispatch documents — structured work definitions that can be consumed by launch scripts to spin up agent worktrees automatically.

This skill is part of the **planning layer** — it activates when `docs/planning/` exists. It reads `docs/planning/threads.md` and `docs/planning/backlog.md` for context when available, but gracefully degrades if they don't exist.

## Convention File Resolution

For dispatch-specific conventions, resolve in this order:

1. **Project-local**: `docs/planning/conventions.md`
2. **Plugin default**: Built-in conventions in this skill (below)

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

Dispatch docs live in `docs/planning/dispatch/` and are the bridge between the backlog (what to do) and agent execution (doing it).

## Parsing Invocation

The user invokes this skill as `/ordvik:dispatch <operation> [args...]`.

| Operation | Arguments |
|-----------|-----------|
| `create`  | `<slug> [--thread N] [--wave N] [--source <ref>]` |
| `list`    | `[--wave N]` |
| `review`  | `<file>` |

If invoked with no arguments, show help. If the user asks conversationally to "write a dispatch doc" or "create tasks for X", treat as `create`.

---

## File Format

### Filename

Pattern: `WWW-TTT-<slug>.md` where:
- `WWW` = wave number (zero-padded, e.g., `006`)
- `TTT` = thread number (zero-padded, e.g., `015`)
- `<slug>` = kebab-case human-readable description

If no wave is specified, omit the wave prefix: `TTT-<slug>.md`.

### YAML Frontmatter (required)

The frontmatter is **machine-parsed** by launch scripts. The `branch` field is required — without it, the doc cannot be launched.

```yaml
---
thread: "015"
wave: "006"
branch: thread/<slug>
executor: agent
scope: search                    # multi-scope repos: which project this targets
source: RFC search/0069
depends_on: []
human_gates:
  - "Task 7: Deploy and verify one service end-to-end"
---
```

Field reference:

| Field | Required | Description |
|-------|----------|-------------|
| `thread` | yes | Thread number from `docs/planning/threads.md` (if it exists) |
| `wave` | no | Wave number if part of a batch |
| `branch` | **yes** | Git branch name — launch scripts create a worktree with this branch |
| `executor` | yes | `agent`, `human`, or `mixed` — launch scripts skip `human` threads |
| `scope` | no | Target scope name from `.ordvik.yml` (multi-scope repos only). Omit for single-scope repos or root-scope work. |
| `source` | yes | Where this work comes from. Use scope-prefixed RFC refs in multi-scope repos (e.g., `RFC search/0069`). |
| `depends_on` | no | Thread numbers that must complete first |
| `human_gates` | no | List of task descriptions that require human execution |
| `platform` | no | `darwin`, `linux`, or `any` (default: `any`) — platform constraint |
| `repo` | no | Cross-repo dispatch target (see below) |

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
  ../monorepo/docs/planning/dispatch/020-widget-api.md"
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
# Thread TTT: Title

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
./scripts/test.sh ./relevant/package/...
./scripts/build.sh relevant-binary
```

**Human validation**: Description of what a human should check after deployment (optional).

---

## Task N (Human Gate): Title

> **Human gate**: Description of why this requires human execution.

Steps the human needs to perform, with commands and checklists.

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

### `create <slug> [--thread N] [--wave N] [--source <ref>]`

1. **Check planning structure** — If `docs/planning/dispatch/` doesn't exist, create it. Note whether `docs/planning/threads.md` and `docs/planning/backlog.md` exist (used for context but not required).

2. **Determine repo model** — Read `.ordvik.yml` if it exists. In multi-scope repos, determine which scope this dispatch targets (from `--source` RFC prefix, from conversation context, or ask the user). Read the scope's config for `paths`, `language`, and `designs` directory.

3. **Gather context** — Use `AskUserQuestion` to understand:
   - What work needs to be done? (or read from the referenced backlog item/RFC)
   - What thread does this belong to? (check `docs/planning/threads.md` if it exists)
   - Is this part of a wave? (check `docs/planning/backlog.md` for wave groupings if it exists)
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

6. **Prompt for verification completeness** — For each task, consider:
   - "How would you know this task is *actually* done, not just 'tests pass'?"
   - "What could go wrong that unit tests wouldn't catch?"
   - "Is there an end-to-end check that would validate the full data flow?"
   - Flag tasks that have only unit-level validation for code that touches multiple subsystems.

7. **Identify human gates** — Mark tasks that require human execution:
   - Deployment to production
   - Operational verification (checking dashboards, logs)
   - Decisions requiring judgment calls
   - Access to systems the agent can't reach

8. **Write the document** — Create the file at `docs/planning/dispatch/WWW-TTT-<slug>.md` following the format above. In multi-scope repos, include the `scope` field in frontmatter. Always include the Pre-PR Checklist and Pre-existing issues sections.

9. **Report** — Show the user: file path, thread/wave numbers, scope (if multi-scope), task count, and any human gates.

### `list [--wave N]`

1. Glob `docs/planning/dispatch/*.md` (exclude wave retros and human-week docs).
2. Parse YAML frontmatter from each file.
3. Output a table: **File**, **Thread**, **Wave**, **Executor**, **Source**, **Branch**.
4. If `--wave` is given, filter to that wave only.

### `review <file>`

1. Read the dispatch document.
2. Check against this checklist:
   - [ ] YAML frontmatter present with required fields (`thread`, `branch`, `executor`, `source`)
   - [ ] `branch` field is set (required for launch scripts)
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

**Don't**:
- Write vague deliverables ("improve the code", "add tests")
- Assume the agent knows which files to modify
- Skip automated validation — every task needs a way to verify it worked
- Combine unrelated changes in one task
- Include tasks that depend on external systems the agent can't access (mark those as human gates)

## Verification Tiers

Each task's validation section should distinguish when appropriate:

- **Automated (agent-run):** Commands the agent runs locally — unit tests, builds, linters, dry-runs
- **Integration (agent-run if infra available):** Commands that require infrastructure — VM-based tests, database tests, network tests. Specify what infrastructure is needed.
- **Operational (human-run):** Checks that require deployment or access the agent doesn't have — dashboard verification, production log checks, user-facing behavior

The pre-PR checklist always requires the full test tier plus relevant integration tests — never the fast tier alone. The fast tier is for iterative development feedback; the full tier is for declaring work complete.

## Inline Markers

Dispatch docs support the same COMMENT/QUESTION marker convention used in RFC authoring. See the RFC authoring guide for the full specification.

During dispatch doc creation, the agent may place `> QUESTION: ...` markers (blockquoted) for decisions that need operator input (e.g., verification approach, scope boundaries, human gate placement). The user may add `COMMENT: ...` markers (plain text, no blockquote) during review. The user responds to QUESTIONs by either editing the text directly or adding a COMMENT. Process both the same way: address, update, remove.
