---
thread: gardening
branch: gardening/api-consistency-YYYY-MM-DD
executor: agent
source: ordvik gardening template
---

# Gardening: API Consistency

**Goal**: Survey API surfaces for naming inconsistencies, undocumented options, and missing validation.

## Context

API surfaces grow organically and accumulate inconsistencies — different naming conventions across endpoints, undocumented flags, missing input validation. This survey inventories the public-facing interfaces and checks for consistency.

---

## Task 1: Enumerate API Surfaces

**Scope**: Build an inventory of all user-facing and inter-service interfaces.

**Method**:
- CLI tools: enumerate all flags, subcommands, and their naming patterns
- Config files: enumerate all config struct fields and their naming conventions
- HTTP endpoints: enumerate routes, methods, and parameter naming
- Message subjects: enumerate NATS/messaging subjects and their naming patterns
- Environment variables: enumerate all env vars the codebase reads

**Deliverables**:
- `docs/planning/gardening/YYYY-MM-DD-api-consistency.md` with the inventory

---

## Task 2: Consistency Analysis

**Scope**: Check the inventory for inconsistencies.

**Method**:
- Naming conventions: are CLI flags consistently kebab-case? Config fields consistently snake_case or camelCase? Is the convention consistent within each surface?
- Undocumented options: are there CLI flags or config fields with no help text or documentation?
- Missing validation: are user-facing inputs validated? Check for fields that accept arbitrary strings but should be constrained (ports, paths, durations, enums)
- Deprecated but not removed: are there options marked deprecated that are still accepted?

**Deliverables**:
- Findings added to the report, categorized:
  - **inconsistency** — naming pattern breaks the convention
  - **undocumented** — option exists but has no description
  - **unvalidated** — user input not checked
  - **deprecated** — should be removed or warned

---

## Task 3: Backlog Items

**Scope**: Create backlog entries for actionable findings.

**Deliverables**:
- Consistency fixes → P2/P3 in backlog
- Validation gaps → P2 in backlog (especially for user-facing inputs)
- Reference the survey report as the source
