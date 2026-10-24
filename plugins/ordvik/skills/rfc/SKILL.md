---
name: rfc
description: Manage RFC lifecycle (create, list, review, status updates) and guide collaborative authoring with structured research and iteration phases.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git config *)
  - Bash(git fetch *)
  - Bash(scripts/rfc *)
  - WebSearch
  - WebFetch
  - Task
  - AskUserQuestion
---

# RFC Management Skill

You are managing RFCs for a software project. This skill uses **convention file discovery** — it checks for project-local files first, falling back to plugin defaults.

## Convention File Resolution

For each reference file, resolve in this order:

1. **Project-local**: `docs/rfcs/conventions.md` (for conventions), `docs/rfcs/template.md` (for template)
2. **Plugin default**: `${CLAUDE_SKILL_DIR}/references/<file>` (built-in defaults)

Read the conventions file first — it determines directory structure, numbering, and scoping. If no project-local file exists, use the plugin default.

Reference files (always read from the plugin — these are not project-customizable):
- `${CLAUDE_SKILL_DIR}/references/review-checklist.md` — review criteria
- `${CLAUDE_SKILL_DIR}/references/authoring-guide.md` — detailed authoring phase guidance

## Repo Model

Ordvik supports two repo models: **single-scope** (one RFC namespace) and **multi-scope** (subprojects with independent RFC numbering). The model is declared in `.ordvik.yml` at the repo root. See `${CLAUDE_SKILL_DIR}/references/conventions.md` for full specification of both models.

**On every invocation**, determine the repo model:
1. Check for `.ordvik.yml` at the repo root. If it exists and declares `scopes`, use the declared scopes.
2. Otherwise, discover scopes from the filesystem (see conventions.md "Filesystem Fallback").
3. If only `docs/rfcs/` exists with no project scopes, treat as single-scope.

## Parsing Invocation

The user invokes this skill as `/ordvik:rfc <operation> [args...]`. Parse the first argument as the operation. Valid operations:

| Operation | Arguments |
|-----------|-----------|
| `author` | `[<short-title>]` |
| `create` | `[--scope <project>] <short-title>` |
| `reserve` | `[--scope <project>] <short-title>` |
| `child` | `<parent-number> <short-title>` |
| `list` | `[--scope <project>] [--status <status>]` |
| `update-status` | `<number> <new-status>` |
| `review` | `<number>` |
| `supersede` | `<old-number> <new-number>` |
| `relate` | `<number> <related-numbers...>` |

If invoked with no arguments or an unrecognized operation, show a brief help summary listing the operations above.

If the user asks to write, draft, or author an RFC conversationally (e.g., "let's write an RFC about X"), treat that as invoking `author` with whatever context they provide.

## Lifecycle States

Valid statuses: `draft`, `considering`, `accepted`, `implemented`, `rejected`, `paused`, `superseded`.

- `accepted` stays while the rollout is in progress; `implemented` means the product shipped.
- **Merging an RFC's PR accepts it.** An RFC is written to be accepted: the PR that lands it sets `status: accepted` in the same commit, so the merge is the operator's acceptance. The exception is an RFC tabled to keep work in progress: its PR says so in the title or first line ("tabled to iterate further") and lands it as `draft` (or `paused`). `considering` is for an RFC whose questions are settled but which waits on something else before acceptance.

## Required Frontmatter Fields

Every RFC must have these bold-markdown frontmatter fields immediately after the `# RFC NNNN: Title` heading:

- `**Status**`: one of the lifecycle states
- `**Author**`: author name
- `**Date**`: creation date in YYYY-MM-DD format

## Optional Frontmatter Fields

These are added as needed, always in this order after the required fields:

- `**Updated**`: YYYY-MM-DD (added/updated on status changes or significant edits)
- `**Parent**`: RFC reference (for child RFCs)
- `**Related**`: comma-separated RFC references
- `**Supersedes**`: RFC reference (on the new RFC)
- `**Superseded by**`: RFC reference (on the old RFC)

RFC references use the format: `RFC NNNN (Short Title)`.

---

## Scope Resolution

Many operations accept an RFC number that may or may not include a scope prefix.

### Scope Registry

Build the scope registry on each invocation:

1. **If `.ordvik.yml` exists** — read the `scopes` section. Each key is a scope name, and its `rfcs` field gives the RFC directory path.
2. **If no `.ordvik.yml`** — discover scopes from the filesystem using `Glob` with pattern `**/docs/rfcs/*.md`. Group results by parent directory:
   - `docs/rfcs/` → root scope
   - `<project>/docs/rfcs/` → project scope named `<project>`
   - `<path>/<project>/docs/rfcs/` → project scope named `<project>`

The scope registry maps each scope name to its RFC directory path and any additional metadata (designs path, source paths, language, numbering overrides).

### Scope Prefix Format

- **Scoped reference**: `<scope>/<number>` (e.g., `search/0005`, `ml/0002`)
- **Unscoped reference**: just `<number>` (e.g., `0038`)

### Resolution Logic

When an RFC number is given **without** a scope prefix:

1. Search all RFC directories in the scope registry for files matching the number.
2. If exactly **one match** → use it.
3. If **multiple matches** → ask the user to disambiguate with a scope prefix. List the matches found with their scope names.
4. If **no matches** → report that no RFC with that number was found.

In **single-scope** repos, unscoped references are always unambiguous.

When an RFC number is given **with** a scope prefix:

1. Look up the scope in the registry.
2. If the scope doesn't exist, report the error and list available scopes.
3. Search only that scope's RFC directory.
4. If not found, report the error.

### Scope Inference for New RFCs

When creating an RFC without `--scope`, infer the scope:

1. **Single-scope repos**: always root scope.
2. **Multi-scope repos**: if the conversation context makes it clear which project is affected, propose that scope. Otherwise, ask the user. Use the `paths` field from `.ordvik.yml` to match discussed files to scopes.

---

## Operations

### `author [<short-title>]`

Enter the full authoring workflow (see **Authoring Workflow** below). The short-title is optional — it will be derived during problem refinement if not provided. If an RFC file already exists for the given title, resume from the current state of that document.

### `create [--scope <project>] <short-title>`

1. Build the scope registry (see Scope Resolution above).
2. **Determine scope:**
   - If `--scope` is given, look it up in the scope registry. Error if unknown.
   - In single-scope repos, use root scope.
   - In multi-scope repos without `--scope`, infer from context or ask the user.
3. **Reserve the number.** If the repo has `scripts/rfc`, run `reserve` (below) and wait until its PR has merged, then start the RFC's own branch from the default branch; the stub and index row are already there. Without `scripts/rfc`, fall back to scanning: take one past the highest number in the scope's directory (accounting for legacy digits, retired numbers and `next_min` in `.ordvik.yml`), and re-scan just before writing in case a concurrent author took it.
4. Get the author name: run `git config user.name` via Bash.
5. Read the template file (project-local `docs/rfcs/template.md` first, then `${CLAUDE_SKILL_DIR}/references/template.md`) and fill in: number, title (convert short-title from kebab-case to Title Case), author, today's date. With a reservation, replace the stub's body and keep its heading.
6. Write the file as `NNNN-<short-title>.md` in the target scope's directory.
7. Report the created file path, RFC number, and scope name.

With a reservation, the RFC's PR edits only its own file and its own index row. It never touches the `Next number` line or other rows, so RFC PRs don't conflict with each other.

### `reserve [--scope <project>] <short-title>`

Takes the next number in a scope with a PR that the repo's `rfc-reservation` workflow merges once the repo's CI passes on it. Needs `scripts/rfc` and `.github/workflows/rfc-reservation.yml` (`/ordvik:init` offers both).

1. Determine the scope as for `create`.
2. `git fetch origin` and start from a fresh default branch. In a cloud session, use the session's own branch, reset to the default branch.
3. Run `scripts/rfc reserve --scope <dir> [--min <next_min>] <short-title> "<Title>"`. It writes the stub `NNNN-<short-title>.md`, appends the row to the scope README's index table, bumps its `Next number` line, and prints the stub's path. Commit nothing else.
4. Commit (`docs(rfc): reserve NNNN <short-title>`), push, and open a ready (not draft) PR containing only that commit.
5. When CI passes on the PR, the workflow checks it with `scripts/rfc verify-reservation`, run from the default branch, and rebase-merges it. If CI passed but the PR is still open, run the workflow by hand with the PR number. If the number was taken on the default branch first, reset and reserve again.
6. Once it has merged, write the RFC on a new PR. A cloud session restarts its branch from the default branch for this, since the reservation PR is merged.

`scripts/rfc next --scope <dir>` prints the next free number without reserving it. `scripts/rfc check` fails on two files sharing a number, unless the scope README's "Number Collisions" table lists it, and on a `Next number` that's already taken; run it in the repo's test suite.

### `child <parent-number> <short-title>`

Same as `create`, but additionally:
1. Resolve the parent RFC using scope resolution.
2. Read the parent RFC to get its title.
3. Add a `**Parent**` field to the new RFC's frontmatter: `RFC <parent-number> (<parent-title>)`.
4. Create the child RFC in the same scope as the parent.

### `list [--scope <project>] [--status <status>]`

1. **Determine which scopes to list:**
   - If `--scope` is given, list only that scope's RFC directory.
   - Otherwise, list all scopes (root + all project scopes).
2. Use `Glob` to find all RFC files across the target scope(s). Exclude README.md files.
3. For each RFC, read the first 10 lines to extract: number, title, status, date.
4. If `--status` is given, filter to only matching RFCs.
5. Output a markdown table with columns: **Scope**, **Number**, **Title**, **Status**, **Date**.
   - Root-scope RFCs show scope as `root`.
   - Project-scope RFCs show the project name (e.g., `leitdar`, `ml`).
6. Sort by scope (root first, then projects alphabetically), then by number ascending within each scope.
7. Group superseded RFCs at the bottom of each scope with an annotation showing what replaced them.

### `update-status <number> <new-status>`

1. Validate the new status is one of the lifecycle states.
2. Resolve the RFC using scope resolution.
3. Use `Edit` to change the `**Status**` field value.
4. Add or update the `**Updated**` field with today's date.
5. Report the change.
6. **If the new status is `accepted`:**
   - Ask the user whether a backlog item should be created for implementation (if `docs/planning/backlog.md` exists).
   - Ask whether any design docs need to be updated or created as a result of this RFC.
   - If yes, identify which design docs are affected (check `docs/designs/` and `<project>/docs/designs/` as appropriate).
   - Note the affected docs for follow-up. Optionally offer to draft the updates now.

### `review <number>`

1. Resolve the RFC using scope resolution.
2. If the RFC has a `**Superseded by**` field, immediately inform the user and redirect to the current version. Do not review superseded RFCs.
3. Read `${CLAUDE_SKILL_DIR}/references/review-checklist.md`.
4. Evaluate the RFC against each criterion in the checklist.
5. If the RFC has a `**Supersedes**` field, include a note about what changed from the previous design (read the old RFC to compare).
6. Output a structured review with: a summary assessment, per-criterion findings (pass/concern/fail), and actionable recommendations.

### `supersede <old-number> <new-number>`

1. Resolve both RFC numbers using scope resolution.
2. Set the old RFC's status to `superseded` using `Edit`.
3. Add `**Superseded by**`: `RFC <new-number> (<new-title>)` to the old RFC's frontmatter.
4. Add `**Supersedes**`: `RFC <old-number> (<old-title>)` to the new RFC's frontmatter.
5. Add or update the `**Updated**` field on the old RFC with today's date.
6. Report both changes.

### `relate <number> <related-numbers...>`

1. Resolve all RFC numbers using scope resolution.
2. For each related number, read that RFC to get its title.
3. Build the related string: comma-separated `RFC NNNN (Title)` entries.
4. If the target RFC already has a `**Related**` field, merge the new entries (avoid duplicates). Otherwise, add the field.
5. Use `Edit` to update the frontmatter.

---

## Authoring Workflow

When the user asks to **write**, **draft**, or **author** an RFC (via `author` operation or implicit detection), follow these five phases. Read `${CLAUDE_SKILL_DIR}/references/authoring-guide.md` for detailed guidance on executing each phase well.

### Phase 1: Problem Refinement

Always runs, regardless of input quality. Even if the user arrives with a problem + solution sketch.

1. **Capture input** — Record whatever the user provides (problem, solution ideas, constraints). If solution ideas are included, acknowledge them but park them for Phase 2.
2. **Separate problem from solution** — If the input mixes problem and solution, extract the core problem. What's broken/missing/painful, independent of any particular fix?
3. **Structured questioning** — Use `AskUserQuestion` to sharpen:
   - Who is affected and how? What's the impact?
   - What's the current state? What workarounds exist?
   - What constraints apply? (compatibility, performance, timeline, dependencies)
   - What does success look like?
   - What are explicit non-goals? (boundary setting)
4. **Determine scope** — Based on which parts of the codebase are affected, propose a scope (root vs. project). Confirm with user.
5. **Draft Problem section** — Write a clear problem statement. Optionally add Goals/Non-Goals if scope needs bounding.
6. **Gate** — Present the problem statement to the user. Mark open questions with `> QUESTION: ...` (blockquoted). Do not proceed to Phase 2 until the user confirms the problem is well-stated and all questions are resolved.

### Phase 2: Solution Research

Always runs. This is where parked solution ideas from Phase 1 become candidates.

1. **Codebase research** — Use Grep, Glob, Read, and Task/Explore agents to understand:
   - Existing patterns and prior art in the codebase
   - Related RFCs (check all scopes)
   - Relevant design docs
   - Code paths that would be affected
2. **External research** — Use WebSearch/WebFetch to verify current docs for any external systems, APIs, libraries, or tools. Never rely on built-in knowledge for external system capabilities.
3. **Identify candidates** — Develop 2-3+ candidate approaches. If the user provided initial solution ideas, evaluate them as candidates alongside others discovered through research.
4. **Document findings** — Add a `## Research Notes` section (temporary, removed before finalizing). Capture what was learned with links to sources.
5. **Present candidates** — Summarize the candidate approaches with trade-offs. Use `AskUserQuestion` to discuss direction with user.
6. **Gate** — User agrees on a direction before detailed drafting begins.

### Phase 3: Drafting

Write the full RFC document.

1. **Create RFC file** — Use the `create` operation (in the appropriate scope) if the file doesn't exist yet. Where the repo has `scripts/rfc`, reserve as soon as the scope is settled in Phase 1, so the number is held while research runs.
2. **Write all sections:**
   - **Problem** — From Phase 1 (already drafted)
   - **Proposal** — Concrete solution based on the chosen direction from Phase 2
   - **Design** — Technical details grounded in research findings. Reference existing code paths, cite sources.
   - **Alternatives Considered** — Other candidates from Phase 2 with genuine trade-offs. No strawman dismissals — each alternative should have real pros that explain why someone might choose it.
   - **Open Questions** — Unresolved design decisions that emerged during drafting. These are genuine uncertainties, not things that should have been resolved in Phase 1. Also place `> QUESTION: ...` markers inline near the relevant design text.
   - **Verification Plan** — Specific, actionable tests and checks.
3. **Identify design doc impacts** — Note which existing design docs would need updating if this RFC is accepted. Add a `## Design Documentation` section listing affected docs (or note that new design docs would need to be created).
4. **Distribute research findings** — The temporary `## Research Notes` from Phase 2 should be emptied:
   - Findings that directly shaped the design → woven into the Design section
   - Detailed evidence, evaluations, or surveys that provide useful background → `## Appendix N: Research — <Topic>` sections appended after the Verification Plan. Use lettered appendices (A, B, C) with numbered subsections (A.1, A.2).
   - Broad research that stands alone and may feed future work → standalone research document in `docs/research/`, referenced from the RFC's `**Related**` field
   - This is a judgment call per session. Not every RFC needs appendices; not every research effort needs a standalone document. The goal is that nothing stays in Research Notes — it all lands somewhere durable.

### Phase 4: Review & Refinement

Iterate until the RFC is solid. See `${CLAUDE_SKILL_DIR}/references/authoring-guide.md` § "Inline Markers" for the full COMMENT/QUESTION convention.

1. **Walk through open questions** — Present each `> QUESTION: ...` (blockquoted, agent-written) to the user, one at a time or in small groups. Resolve them by updating the Design section and removing the marker.
2. **Process inline comments** — Scan the document for `COMMENT: ...` markers (plain text, user-written). For each: address the feedback, update the text, remove the marker, and summarize what changed.
3. **Integrate conversational feedback** — Handle feedback given in conversation the same way — update the document, not just the discussion.
4. **Repeat** — Continue until the user is satisfied, all markers are resolved, and no open questions remain.

### Phase 5: Finalization

Land the RFC and handle downstream effects.

1. **Final cleanup** — Remove any remaining markers (COMMENT, QUESTION, Research Notes). Ensure all sections are complete.
2. **Review pass** — Quick self-review against the review checklist (`${CLAUDE_SKILL_DIR}/references/review-checklist.md`). Flag any remaining concerns.
3. **Design doc plan** — If design doc impacts were identified in Phase 3, confirm the plan for updating them.
4. **Set the status the PR lands with.** By default the RFC goes up as `accepted`, since merging its PR is the acceptance (see Lifecycle States). If the operator wants to table it and keep iterating later, it goes up as `draft` and the PR says it is tabled. Then offer to begin implementation (a plan and dispatches) once it has merged.
5. **Commit** — Offer to commit the RFC (and any design doc updates) with an appropriate commit message.
