---
thread: gardening
branch: gardening/dependency-health-YYYY-MM-DD
executor: agent
source: ordvik gardening template
---

# Gardening: Dependency Health

**Goal**: Assess the health of external dependencies — outdated versions, unused imports, deprecated APIs, security advisories.

## Context

Dependencies drift over time. This survey identifies what needs attention before it becomes a problem.

---

## Task 1: Survey

**Scope**: Audit all external dependencies.

**Method**:
- Check for outdated dependencies using language-specific tools:
  - Go: `go list -m -u all`
  - Python: `pip list --outdated` or `uv pip list --outdated`
  - Node: `npm outdated`
  - Rust: `cargo outdated`
- Find unused imports and dependencies (imported but never referenced)
- Search for deprecated API usage patterns (check changelogs of major dependencies)
- Check for known security advisories:
  - Go: `govulncheck ./...`
  - Python: `pip-audit` or `safety check`
  - Node: `npm audit`

**Deliverables**:
- `<planning>/gardening/YYYY-MM-DD-dependency-health.md` with findings categorized:
  - **update-now** — patch/minor version bump, low risk
  - **plan-update** — major version bump or API changes, needs dispatch
  - **security** — known vulnerability, prioritize
  - **remove** — unused dependency, safe to drop
  - **monitor** — no action now, but watch for changes

**Automated validation**: Report file exists and is non-empty.

---

## Task 2: Backlog Items

**Scope**: Create backlog entries for actionable findings.

**Deliverables**:
- Security items → P1 in backlog
- Plan-update items → P2 in backlog
- Reference the survey report as the source
