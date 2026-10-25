---
thread: gardening
branch: gardening/refactoring-survey-YYYY-MM-DD
executor: agent
source: ordvik gardening template
---

# Gardening: Refactoring Survey

**Goal**: Identify consolidation opportunities across the codebase.

## Context

Scan for repeated patterns that could be shared abstractions. This is discovery work — report findings, don't fix them. Trivial fixes (unused imports, obvious dead code) can be included in the PR if clearly separated from findings.

---

## Task 1: Survey

**Scope**: Scan the codebase for structural duplication and consolidation candidates.

**Method**:
- Search for structurally similar functions across packages (same parameter shapes, similar logic)
- Identify types that duplicate fields or methods from other types
- Flag packages that import the same set of dependencies and implement similar logic
- Check for copy-pasted code with minor variations (string constants, field names differ but structure is identical)
- Look for inconsistent naming conventions across packages (same concept, different names)

**Deliverables**:
- `<planning>/gardening/YYYY-MM-DD-refactoring-survey.md` with findings, each tagged as:
  - **consolidate** — clear candidate for shared abstraction
  - **investigate** — might be worth consolidating, needs human judgment
  - **intentional** — looks duplicated but has good reasons to stay separate

**Automated validation**: Report file exists and is non-empty.

---

## Task 2: Backlog Items

**Scope**: Create backlog entries for actionable findings.

**Deliverables**:
- Add items to `<planning>/backlog.md` at P2/P3 for each **consolidate** finding
- Reference the survey report as the source
