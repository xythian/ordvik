---
thread: gardening
branch: gardening/rfc-hygiene-YYYY-MM-DD
executor: agent
source: ordvik gardening template
---

# Gardening: RFC Hygiene

**Goal**: Detect and fix RFC numbering collisions, stale cross-references, and metadata inconsistencies.

## Context

In repos with active RFC authoring — especially multi-scope repos or repos with concurrent agent sessions — numbering collisions and stale references can accumulate. This survey detects these mechanical issues and fixes them.

---

## Task 1: Collision Detection

**Scope**: Find RFC number collisions within each scope.

**Method**:
- For each RFC scope (root + all project scopes), scan filenames for duplicate numbers
- A collision is two files with the same numeric prefix in the same scope (e.g., `0042-feature-a.md` and `0042-feature-b.md`)
- For each collision: determine which file is newer (git log or file date), propose renumbering the newer one to the next available number

**Deliverables**:
- List of collisions found (if any)
- For each: proposed renumbering with the specific filename change

**Automated validation**: No duplicate numbers within any scope.

---

## Task 2: Fix Collisions

**Scope**: Renumber colliding RFCs and update all references.

**Method**:
- Rename the newer file to the next available number
- Grep the entire repo for references to the old number (in RFC cross-references, dispatch docs, backlog, design docs, code comments)
- Update all references to use the new number
- Update the RFC's own title heading (`# RFC NNNN:`)

**Deliverables**:
- Renamed files
- Updated cross-references (list each file changed)

**Automated validation**: Grep for old number returns no results.

---

## Task 3: Cross-Reference Audit

**Scope**: Verify all RFC cross-references are valid.

**Method**:
- Find all `**Related**`, `**Parent**`, `**Supersedes**`, `**Superseded by**` fields across all RFCs
- For each reference: verify the target RFC exists and the title matches
- Find references to RFCs in dispatch docs (`source: RFC NNNN`), backlog items, and design docs
- Flag broken references (target doesn't exist) and stale titles (title changed)

**Deliverables**:
- Findings added to report
- Fix-now items: update stale titles, fix broken references

---

## Task 4: Metadata Consistency

**Scope**: Check RFC frontmatter for consistency issues.

**Method**:
- Verify all RFCs have required fields: Status, Author, Date
- Check that Status values are valid lifecycle states
- Flag RFCs with `superseded` status but no `**Superseded by**` field (or vice versa)
- Check date formats (should be YYYY-MM-DD)

**Deliverables**:
- Findings added to report
- Fix-now items: add missing fields, fix formats
