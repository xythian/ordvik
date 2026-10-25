---
name: garden
description: Gardening — discovery-oriented codebase surveys. Lists gardening templates, helps customize and launch surveys, and reviews past findings for trends.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git log *)
  - Bash(git diff *)
  - Bash(./scripts/*)
  - Task
  - AskUserQuestion
---

# Gardening Skill

**Where planning lives.** `<planning>` below is the project's planning directory: `projects/<project>/` in the planning repository when `.ordvik.yml` names one (`planning: {repo: <owner>/<planning-repo>, project: <project>}`; cloned beside the code checkout as `planning/`, or at `$ORDVIK_PLANNING`), else `docs/planning/` in the repository. In the planning repository the backlog is one file per item in `backlog/` (ordered by `priority`, then `rank`) and threads are files in `threads/`, where `docs/planning/` has `backlog.md` and `threads.md`; status reports are in `status/` (else `docs/status/`); dispatch docs carry `id:`, `thread:` and `repo:` (else the id is `thread:`); plans list `dispatches:` (else `threads:`); and a change is a commit straight to its `main` (fetch, change, `scripts/plan check`, commit, push), not a PR. `scripts/plan` and `scripts/asks` read either layout (in the planning repository they run `ordvik planning`).

Gardening is recurring, discovery-oriented work — agents dispatched to survey the codebase and identify work that needs doing. The primary output is information (findings reports and backlog items), not code changes.

## Parsing Invocation

The user invokes as `/ordvik:garden [operation]`.

| Operation | Description |
|-----------|-------------|
| (none) | List available gardening templates |
| `survey <template>` | Generate a gardening dispatch doc from a template |
| `review` | Review past gardening reports for trends |

## Available Templates

Template files are in `${CLAUDE_PLUGIN_ROOT}/templates/gardening/`. Each is a complete dispatch doc that can be customized and launched.

| Template | Purpose |
|----------|---------|
| `refactoring-survey` | Identify repeated patterns that could be consolidated — duplicated code, similar-but-different implementations, inconsistent naming |
| `unfinished-work` | Find TODOs, FIXMEs, commented-out code, stub implementations, skipped tests without explanation, broken baseline |
| `dependency-health` | Check for outdated dependencies, unused imports, deprecated API usage, security advisories |
| `test-coverage` | Identify untested code paths, packages with no tests, tests that don't assert meaningfully, missing integration tests |
| `doc-drift` | Find design docs referencing changed code, RFCs with accepted status but no implementation, stale AGENTS.md guidance, outdated comments |
| `api-consistency` | Survey API surfaces (CLI flags, config fields, endpoints) for naming inconsistencies, undocumented options, missing validation |
| `rfc-hygiene` | Detect RFC numbering collisions, stale cross-references, and metadata inconsistencies |

## Survey Operation

`/ordvik:garden survey <template>`

1. **Select and customize** — Read the template, ask the user:
   - Scope: full repo or specific directories/scopes?
   - Focus areas: any known problem spots to emphasize?
   - Depth: quick scan or thorough investigation?

2. **Generate dispatch doc** — Create a gardening dispatch doc at `<planning>/dispatch/` following these conventions:

   **Frontmatter:**
   ```yaml
   ---
   thread: gardening
   branch: gardening/<template>-YYYY-MM-DD
   executor: agent
   source: ordvik gardening template
   ---
   ```

   **Branch naming:** `gardening/<template>-YYYY-MM-DD` — date-stamped because gardening is periodic.

   **Output location:** Findings go to `<planning>/gardening/` — create if it doesn't exist.

   **Task structure:** Each template has two standard tasks:

   - **Task 1: Survey** — scan the codebase using the template's methodology. Write findings to `<planning>/gardening/YYYY-MM-DD-<template>.md`. Categorize each finding:
     - **fix-now** — trivial, include in this PR
     - **backlog** — needs its own dispatch, add to backlog
     - **investigate** — unclear, needs human judgment
     - **intentional** — looks like an issue but has good reasons

   - **Task 2: Backlog items** — create backlog entries at P2/P3 for actionable findings. Reference the survey report as source.

3. **Report** — show the generated dispatch doc path and suggest: "Launch this with your worktree script, or run the survey directly in this session."

## Template Details

### refactoring-survey

Survey methodology:
- Search for structurally similar functions across packages
- Identify types that duplicate fields/methods from other types
- Flag packages that import the same dependencies and implement similar logic
- Check for copy-pasted code with minor variations

### unfinished-work

Survey methodology:
- Grep for TODO, FIXME, HACK, XXX, TEMP markers
- Find commented-out code blocks (>3 lines)
- Find tests marked with skip/pending without explanation
- Find empty catch/error blocks
- Run `./scripts/test.sh full` and record failures/skips
- Run `./scripts/test.sh list` and run each available integration suite
- Run build and record warnings
- Check for tests guarded by skip that reference resolved issues

Includes the **broken baseline survey**: catalogue pre-existing test failures, build warnings, and lint issues. Categorize as fix-now, backlog, or investigate.

### dependency-health

Survey methodology:
- Check for outdated dependencies (language-specific: `go list -m -u all`, `npm outdated`, `pip list --outdated`)
- Find unused imports/dependencies
- Search for deprecated API usage patterns
- Check for known security advisories

### test-coverage

Survey methodology:
- Identify packages/modules with no test files
- Find test files that exist but have no assertions
- Identify code paths at subsystem boundaries with no integration tests
- Compare test coverage against changed-frequently files (git log --stat)

### doc-drift

Survey methodology:
- Cross-reference design docs against current code (do referenced functions/types still exist?)
- Find accepted RFCs with no corresponding implementation
- Check AGENTS.md/CLAUDE.md guidance against current practice
- Find comments that reference renamed/deleted code

### api-consistency

Survey methodology:
- Enumerate CLI flags, config fields, and API endpoints
- Check naming conventions (kebab-case vs snake_case vs camelCase consistency)
- Find undocumented options
- Check for validation on user-facing inputs

## Review Operation

`/ordvik:garden review`

1. Read all files in `<planning>/gardening/`
2. Compare findings across time — are the same issues recurring?
3. Track trends: is the codebase getting cleaner or dirtier?
4. Identify patterns: which types of issues keep appearing?
5. Present a summary with recommendations for the next gardening focus

## Gardening Conventions

- **Survey and report, don't fix.** Fixes go through the normal dispatch pipeline. Exception: trivial fixes (typos, unused imports) can be included in the gardening PR, clearly separated from findings.
- **Frequency is operator-driven.** Gardening runs are dispatched when there's capacity or suspected drift, not on a schedule. The planning skill can suggest gardening runs during gaps between waves.
- **Findings accumulate.** The `<planning>/gardening/` directory is an input for planning sessions. Past surveys inform future priorities.
