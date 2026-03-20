---
thread: gardening
branch: gardening/test-coverage-YYYY-MM-DD
executor: agent
source: ordvik gardening template
---

# Gardening: Test Coverage

**Goal**: Identify untested code paths, packages with no tests, and missing integration tests at subsystem boundaries.

## Context

Test coverage gaps tend to accumulate at boundaries — where subsystems interact, where config flows through multiple layers, where error paths are rare. This survey focuses on structural coverage gaps, not line-by-line coverage metrics.

---

## Task 1: Survey

**Scope**: Identify structural testing gaps across the codebase.

**Method**:
- Identify packages/modules with no test files at all
- Find test files that exist but have no meaningful assertions (empty tests, tests that only check "no error")
- Identify code paths at subsystem boundaries with no integration tests:
  - Config generation → config consumption
  - Service startup → health check
  - Data write → data read across services
- Compare test coverage against frequently-changed files (`git log --stat` for last 3 months) — high-churn files with low test coverage are the highest-risk gaps
- Check for missing edge case tests: nil/empty inputs, error paths, boundary conditions

**Deliverables**:
- `docs/planning/gardening/YYYY-MM-DD-test-coverage.md` with findings categorized:
  - **critical-gap** — high-churn code with no tests, or subsystem boundary with no integration test
  - **missing-tests** — package/module with no test files
  - **weak-tests** — tests exist but don't assert meaningfully
  - **edge-cases** — specific untested scenarios worth adding

**Automated validation**: Report file exists and is non-empty.

---

## Task 2: Backlog Items

**Scope**: Create backlog entries for critical gaps.

**Deliverables**:
- Critical-gap items → P1/P2 in backlog
- Missing-tests items → P2/P3 in backlog
- Reference the survey report as the source
