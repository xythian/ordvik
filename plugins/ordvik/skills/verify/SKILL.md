---
name: verify
description: Review completed work for a thread or a dispatch plan. Runs automated validation, checks completion criteria, and performs cross-cutting consistency checks.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git log *)
  - Bash(git diff *)
  - Bash(git status *)
  - Bash(./scripts/*)
  - Task
  - AskUserQuestion
---

# Verification Skill

Reviews completed work for a thread or a dispatch plan. Runs automated validation, checks completion criteria, and identifies cross-cutting consistency issues. Can be invoked manually or dispatched as an audit thread.

## Parsing Invocation

The user invokes as `/ordvik:verify <target>`.

| Target | Description |
|--------|-------------|
| `<dispatch-doc>` | Verify a single thread's dispatch doc |
| `--plan <slug>` | Verify all finished threads in a dispatch plan |
| `--branch <name>` | Verify work on a specific branch against its dispatch doc |

## Single Thread Verification

### Level 2: Thread-Level Review

For a single dispatch doc:

1. **Read the dispatch doc** — parse frontmatter, tasks, completion criteria, and pre-PR checklist.

2. **Check task completion** — for each task:
   - Is the task marked as done?
   - Do the deliverables exist? (check file paths, function names, types mentioned)
   - Run the automated validation commands from the task
   - If the task is marked `(Audit)`: verify its automated validation commands pass. Flag if the audit task body references earlier task numbers by outcome (pattern: "Task N created/implemented/added") — this indicates it was not self-contained.
   - If the task is marked `(Contract)`: record the deliverable file paths as the "contract files" for step 3.
   - For non-contract tasks that list a **Contract** field: verify that the contract files have NOT been modified since the contract task's commit. Use `git log --oneline -- <contract-file>` to check if the file was touched after the contract task was marked done.

3. **Contract integrity check** — if the thread contains `(Contract)` tasks:
   - Identify contract deliverable files from the contract task's Deliverables section.
   - Check `git log` for modifications to those files after the contract task commit.
   - If modified: FAIL with "Contract violation — test files were modified during implementation."
   - Run the contract test files independently to confirm they pass with the full test suite.

4. **Run the pre-PR checklist**:
   - [ ] Full test suite: `./scripts/test.sh full` (not the fast tier)
   - [ ] Relevant integration tests: check `./scripts/test.sh list` for applicable suites
   - [ ] Full build: `./scripts/build.sh`
   - [ ] All completion criteria verified
   - [ ] No stale references to removed/renamed code

5. **Stale reference check** — scan for:
   - Comments referencing deleted/renamed code
   - Variable names that reference old concepts
   - Doc strings that describe removed behavior
   - Import statements for deleted packages

6. **Generate findings report**:

```markdown
# Verification: [Thread Title]

**Dispatch doc**: [path]
**Date**: YYYY-MM-DD
**Result**: PASS | PASS WITH NOTES | FAIL

## Task Verification

| Task | Deliverables | Validation | Status |
|------|-------------|------------|--------|
| 1: ... (Contract) | ✓ tests written | ✓ files unmodified | PASS |
| 2: ... | ✓ all present | ✓ contract tests pass | PASS |
| 3: ... | ✓ all present | ✗ build warning | NOTE |
| 4: ... (Audit) | ✓ self-contained | ✓ validation passes | PASS |

## Pre-PR Checklist

- [x] Full test suite passes
- [x] Integration tests pass
- [x] Build succeeds
- [ ] Stale reference found in X (see below)

## Findings

### Issues
- [severity: high/medium/low] Description of issue

### Stale References
- [file:line] — description

## Recommendation

[PASS: ready for PR | NEEDS FIXES: list what | NEEDS REVIEW: list concerns]
```

## Plan Verification

### Level 3: Plan-Level Audit

For a plan (`--plan <slug>`):

1. **Identify all threads** — `scripts/plan status <slug> --json`; audit the `done` ones (and `started` ones on request).

2. **Run Level 2 for each thread** — individual thread verification.

3. **Cross-cutting consistency checks**:
   - **Naming conventions** — do new types/functions/packages follow existing naming patterns across all threads?
   - **Shared interfaces** — if multiple threads touch the same interfaces, are the changes consistent?
   - **Config patterns** — do config structs, default values, and parameter functions follow the same patterns across threads?
   - **Error handling** — consistent error wrapping, logging levels, and error types?
   - **Test patterns** — similar test structure and assertion patterns across threads?
   - **Audit coverage** — do threads with 4+ implementation tasks include audit tasks? Flag threads that lack them.

4. **Data flow tracing** — for threads that touch related subsystems:
   - Trace data flows end-to-end across changed code paths
   - Check that types match at boundaries
   - Verify error propagation across service boundaries

5. **Generate plan audit report**:

```markdown
# Plan <slug> Audit

**Date**: YYYY-MM-DD
**Threads audited**: [list]
**Result**: PASS | ISSUES FOUND

## Per-Thread Results

| Thread | Result | Issues |
|--------|--------|--------|
| 015: ... | PASS | None |
| 016: ... | NOTE | 1 stale reference |

## Cross-Cutting Findings

### Consistency Issues
- [Description, affected threads, suggested fix]

### Data Flow Issues
- [Description, affected paths]

## Disposition

| Finding | Severity | Disposition |
|---------|----------|-------------|
| Config drift in X | Medium | Fixed (commit abc123) |
| Missing DefaultParams in Y | High | Deferred (backlog item created) |
| Naming inconsistency in Z | Low | Accepted (documented rationale) |

## Remaining Work

- [Backlog items created for deferred findings]
- [Follow-up dispatch docs if needed]
```

Every finding must be categorized as:
- **Fixed** — with the specific commit
- **Deferred** — with a backlog item created
- **Accepted** — with documented rationale

"Found and documented" is not the same as "fixed."
