---
thread: gardening
branch: gardening/unfinished-work-YYYY-MM-DD
executor: agent
source: ordvik gardening template
---

# Gardening: Unfinished Work

**Goal**: Catalogue incomplete work, accumulated rot, and broken baseline issues.

## Context

Find TODOs, stubs, skipped tests, and pre-existing failures. This survey produces a clean inventory of technical debt and baseline health issues.

---

## Task 1: Marker Survey

**Scope**: Find all incomplete-work markers in the codebase.

**Method**:
- Grep for TODO, FIXME, HACK, XXX, TEMP markers
- Find commented-out code blocks (>3 lines of contiguous commented code)
- Find stub implementations (functions that return nil/zero/empty without real logic)
- Find tests marked with skip/pending — flag those without an explanation or with references to issues that may be resolved

**Deliverables**:
- `docs/planning/gardening/YYYY-MM-DD-unfinished-work.md` with categorized findings

**Automated validation**: Report file exists and is non-empty.

---

## Task 2: Broken Baseline Survey

**Scope**: Identify tests, builds, or lint checks that fail on the current main branch.

**Method**:
- Run `./scripts/test.sh full` and record any failures or skips without explanations
- Run all integration test suites (`./scripts/test.sh list` to enumerate, then run each available suite)
- Run `./scripts/build.sh` and record warnings
- Check for tests guarded by skip that reference issues or TODOs that may be resolved

**Deliverables**:
- Findings report with each issue categorized:
  - **fix-now** — trivial to fix, include in this PR
  - **backlog** — needs its own dispatch, add to backlog
  - **investigate** — unclear if it's a real problem
- Fix-now items fixed in this PR
- Backlog items added to `docs/planning/backlog.md` at P2

**Automated validation**: `./scripts/test.sh full` and `./scripts/build.sh` pass (after fix-now items).

---

## Task 3: Backlog Items

**Scope**: Create backlog entries for actionable findings from both tasks.

**Deliverables**:
- Add items to `docs/planning/backlog.md` at P2/P3
- Reference the survey report as the source
