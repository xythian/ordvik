---
thread: gardening
branch: gardening/doc-drift-YYYY-MM-DD
executor: agent
source: ordvik gardening template
---

# Gardening: Documentation Drift

**Goal**: Find documentation that has drifted from the code it describes — stale design docs, unimplemented accepted RFCs, outdated agent guidance, dead comments.

## Context

Documentation rots when code changes but docs don't. This survey cross-references documentation against the current codebase to find mismatches.

---

## Task 1: Design Doc Drift

**Scope**: Check design docs against current code.

**Method**:
- Read each file in `docs/designs/` (and `<scope>/docs/designs/` in multi-scope repos)
- For each function, type, or file path referenced in a design doc, verify it still exists
- Flag sections that describe architecture or data flow that no longer matches the code
- Check for design docs that reference RFCs — verify the RFC status is consistent

**Deliverables**:
- Findings in `<planning>/gardening/YYYY-MM-DD-doc-drift.md`

---

## Task 2: RFC Implementation Status

**Scope**: Find accepted RFCs with no corresponding implementation.

**Method**:
- List all RFCs with `**Status**: accepted`
- For each, check whether the proposed changes exist in the codebase
- Flag accepted RFCs where implementation appears incomplete or missing
- Check for RFCs in `considering` status that have been sitting without activity

**Deliverables**:
- Findings added to the doc-drift report

---

## Task 3: Agent Guidance Drift

**Scope**: Check CLAUDE.md / AGENTS.md against current practice.

**Method**:
- Read CLAUDE.md and AGENTS.md (if they exist)
- Verify referenced file paths, commands, and conventions still exist and work
- Flag guidance that contradicts current code patterns
- Check for stale build/test commands

**Deliverables**:
- Findings added to the doc-drift report

---

## Task 4: Stale Comments

**Scope**: Find code comments that reference changed or deleted code.

**Method**:
- Search for comments referencing function/type names that no longer exist
- Find TODO/FIXME comments referencing completed work or merged PRs
- Flag comments that describe behavior the code no longer exhibits

**Deliverables**:
- Findings added to the doc-drift report, categorized:
  - **fix-now** — trivial comment update or removal
  - **backlog** — design doc needs rewrite
  - **investigate** — unclear if doc or code is wrong

---

## Task 5: Backlog Items

**Scope**: Create backlog entries for actionable findings.

**Deliverables**:
- Design doc rewrites → P2 in backlog
- Unimplemented accepted RFCs → flag for planning session
- Reference the survey report as the source
