# RFC Conventions — Default

Plugin default conventions. Projects customize by creating `docs/rfcs/conventions.md` which takes precedence over this file.

## Repo Models

Ordvik supports two repo models as first-class patterns. The model is declared in `.ordvik.yml` at the repo root (or inferred from directory structure).

### Single-Scope

The default. One RFC namespace, one set of design docs, one codebase.

```yaml
# .ordvik.yml
ordvik:
  model: single-scope

scopes:
  root:
    rfcs: docs/rfcs/
    designs: docs/designs/
```

- All RFCs live in `docs/rfcs/` with a single numbering sequence
- No scope prefixes needed — `0015` is unambiguous
- `--scope` flag is accepted but unnecessary

### Multi-Scope

For monorepos where subprojects have independent concerns, RFC histories, and design docs. Each scope is a named project unit with its own RFC numbering.

```yaml
# .ordvik.yml
ordvik:
  model: multi-scope

scopes:
  root:
    rfcs: docs/rfcs/
    designs: docs/designs/
    research: docs/research/
    description: Cross-cutting concerns, shared infrastructure, monorepo tooling
  search:
    rfcs: search/docs/rfcs/
    designs: search/docs/designs/
    research: search/docs/research/
    paths: [search/]
    language: go
  monitor:
    rfcs: monitor/docs/rfcs/
    designs: monitor/docs/designs/
    research: monitor/docs/research/
    paths: [monitor/]
    language: go
  ml:
    rfcs: python/ml/docs/rfcs/
    designs: python/ml/docs/designs/
    research: python/ml/docs/research/
    paths: [python/ml/]
    language: python
```

**Scope fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `rfcs` | yes | Path to the scope's RFC directory (relative to repo root) |
| `designs` | no | Path to the scope's design doc directory |
| `research` | no | Path to the scope's research doc directory |
| `paths` | no | Source directories this scope owns (for scope inference) |
| `language` | no | Primary language (for build/test script selection) |
| `description` | no | What this scope covers |
| `numbering` | no | Override numbering format (see Legacy Numbering below) |

**Key properties of multi-scope:**

- Each scope has **independent numbering** — `search/0005` and root `0005` are different RFCs
- The **root scope** is always present and is the default when no scope is specified
- Scopes are **declared**, not just discovered — the `.ordvik.yml` file is the registry
- New scopes are added by editing `.ordvik.yml` and creating the directory
- Scope names are used as **prefixes** in cross-scope references: `search/0005`, `ml/0002`

**Scope determination heuristics** (when `--scope` is not given during authoring):

| Signal | Likely Scope |
|--------|-------------|
| Changes only files under one scope's `paths` | That scope |
| Modifies files in multiple scopes' `paths` | Root scope |
| Modifies shared packages not owned by any scope | Root scope |
| Changes build system, CI, or repo-level tooling | Root scope |
| New feature entirely within one project | That project's scope |

Cross-cutting concerns that primarily affect one scope but have wider implications go in the primary scope, with the Design section documenting the cross-cutting impact.

### Filesystem Fallback

If no `.ordvik.yml` exists, scopes are discovered from the filesystem:

1. Use `Glob` with pattern `**/docs/rfcs/*.md`
2. Group results by parent directory
3. `docs/rfcs/` → root scope
4. `<project>/docs/rfcs/` → project scope named `<project>`
5. `<path>/<project>/docs/rfcs/` → project scope named `<project>` (nested paths like `python/ml/` use the last directory component)

This fallback means ordvik works in repos that haven't created `.ordvik.yml` yet. But declared scopes are preferred — they're explicit, support `paths` for scope inference, and avoid surprising discoveries.

## Numbering

- **Standard**: 4-digit, zero-padded sequence numbers (e.g., `0001`, `0037`)
- Filename format: `NNNN-short-title.md`
- Take numbers with a reservation PR (`scripts/rfc reserve`, the skill's `reserve`), which the `rfc-reservation` workflow merges once CI passes on it. Scanning for "highest + 1" is the fallback for repos without `scripts/rfc`: parallel authors collide that way, and RFC PRs that each append an index row conflict with each other.
- `scripts/rfc next` prints one past the highest file, index row, or `Next number` line in the scope; `--min` applies `next_min`.
- Duplicate numbers that predate reservations go in the scope README's "Number Collisions" table; `scripts/rfc check` fails on any other.
- Each scope has independent numbering

### Legacy Numbering

Some scopes may have historical files with non-standard numbering (e.g., 3-digit). Declare this in `.ordvik.yml`:

```yaml
scopes:
  search:
    rfcs: search/docs/rfcs/
    numbering:
      legacy_digits: 3    # Existing files use 001, 002, etc.
      # New RFCs still use 4-digit: 0014-new-feature.md
```

Existing files keep their current names — no renaming. New RFCs in scopes with legacy numbering use 4-digit numbering. The skill accounts for both patterns when scanning for the next number.

### Retired Numbers

Scopes can declare retired numbers — numbers that were previously used and should not be reused:

```yaml
scopes:
  root:
    rfcs: docs/rfcs/
    numbering:
      retired: [1-6, 12-38]    # Moved to project scopes
      next_min: 41              # Skip to here
```

## Frontmatter Format

Frontmatter uses bold-markdown fields (not YAML front matter), placed immediately after the title heading:

```markdown
# RFC NNNN: Title

**Status**: draft
**Author**: Author Name
**Date**: 2026-01-30
```

## Author Default

Use `git config user.name` to get the author name.

## Optional Sections

Beyond the required sections (Problem, Proposal, Design, Alternatives Considered, Verification Plan), RFCs commonly include:

- **Goals** / **Non-Goals** — when scope needs explicit boundaries
- **Open Questions** — for unresolved design decisions
- **Migration** / **Rollout** — for changes to existing systems
- **Security Considerations** — when the design has security implications
- **Design Documentation** — which design docs are affected and need updating

## Design Documentation

### Purpose

`docs/designs/` (or `<scope>/docs/designs/` in multi-scope repos) contains living architecture documents that describe the current state of the system. RFCs capture decisions; design docs capture the resulting architecture.

### Structure

- **Single-scope**: `docs/designs/` for architecture, `docs/research/` for research
- **Multi-scope**: `docs/designs/` and `docs/research/` for cross-cutting; `<scope>/docs/designs/` and `<scope>/docs/research/` for project-specific

### Research Documents

`docs/research/` (or `<scope>/docs/research/`) contains investigation and evaluation documents — technology surveys, ecosystem assessments, codebase analyses. Research documents inform decisions but don't propose work.

Research can also live as RFC appendices (`## Appendix N: Research — <Topic>`) when tightly scoped to a specific RFC. See the research skill for the judgment call between standalone docs and appendices.

### RFC–Design Doc Integration

During RFC authoring:
1. Identify which existing design docs the RFC would affect
2. Note whether new design docs would need to be created
3. Add a `## Design Documentation` section to the RFC listing affected docs

When an RFC is accepted:
1. Update the relevant design docs to reflect the new architecture
2. If a new design doc is needed, create it
3. Design docs should reference the RFC that motivated the change
