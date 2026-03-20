# RFC Authoring Guide

Detailed guidance for executing the authoring workflow phases defined in SKILL.md. That file defines _what_ to do; this guide explains _how_ to do it well.

## Problem Refinement Techniques

### Separating Problem from Solution

Users often arrive with a solution in mind. The goal is to extract the underlying problem so that all solutions (including the user's) get a fair evaluation.

**Techniques:**

- **The "Why" ladder** — When the user states a solution, ask "What problem does this solve?" Then ask "Why is that a problem?" Repeat until you reach a root cause or pain point.
- **The "Without" test** — Describe the world without the proposed solution. What's painful? That's the problem.
- **Restate without technology** — If the user says "we need a Redis cache," restate as "response times are too slow for the read pattern" or "repeated computation is wasteful." The technology-free statement is the problem.

When the user provides both problem and solution, acknowledge the solution explicitly: "I've noted your solution idea — we'll evaluate it alongside other approaches in the research phase." This prevents the user from feeling their input was discarded.

### Question Sequences by Input Type

**Vague idea** ("we should improve the backup system"):
1. What specifically isn't working? What triggered this thought?
2. Who is affected? How often?
3. What does the current system do? Where does it fall short?
4. What would "improved" look like concretely?

**Problem sketch** ("backups are slow and sometimes fail silently"):
1. How slow? What's the current time vs. acceptable time?
2. What does "fail silently" mean — no error, wrong exit code, partial backup?
3. How often does this happen? Is there a pattern?
4. What's the blast radius of a failed backup?

**Problem + solution** ("backups fail silently, so we should add health checks"):
1. *(Park the solution)* Let's make sure we understand the failure modes first.
2. What kinds of failures happen? (corruption, timeout, partial, network)
3. How are failures discovered today? How long until someone notices?
4. What's the recovery process when a failure is found?
5. Are there other symptoms beyond silent failures?

### Identifying Non-Goals

Non-goals bound the RFC and prevent scope creep. Good non-goals are things a reasonable reader might expect to be in scope.

- **Ask:** "What might someone assume this RFC covers, but it doesn't?"
- **Ask:** "What related problems are we explicitly deferring?"
- **Test:** If removing a non-goal wouldn't surprise anyone, it's not worth listing.

### Signs the Problem Statement Is Ready

- A reader unfamiliar with the project can understand the pain
- The problem is stated without reference to any particular solution
- Impact is quantified or at least characterized (who, how often, how severe)
- Constraints are explicit
- Non-goals clearly bound the scope
- No QUESTION or COMMENT markers remain

## Research Methodology

### Codebase Research

**What to search for:**

1. **Existing patterns** — How does the codebase handle similar problems? Use `Grep` for key function/type names, `Glob` for file patterns.
2. **Related RFCs** — Search all scopes (`docs/rfcs/`, `*/docs/rfcs/`, `*/*/docs/rfcs/`). Read titles and statuses. Any accepted RFC in the same area is important context.
3. **Design docs** — Check `docs/designs/` and `<project>/docs/designs/` for existing architecture docs. These describe the current state.
4. **Affected code paths** — Identify the code that would change. Understand its current structure, dependencies, and test coverage.

**When to use Task/Explore agents vs. direct Grep/Glob:**

- **Direct Grep/Glob:** When you know what you're looking for — specific function names, file paths, patterns.
- **Task/Explore agent:** When you need broad understanding — "how does the backup system work?" or "what are all the places that touch VM lifecycle?" Use this for investigations that would require more than 3-4 search queries.

### External Research

**Critical rule: always verify, never assume.** Your built-in knowledge may be stale.

- **APIs and libraries:** Check current documentation for version compatibility, available methods, configuration options.
- **Tools:** Verify CLI flags, supported formats, and platform compatibility against current docs.
- **Standards:** Confirm protocol versions, required fields, and compatibility requirements.
- **Use WebSearch first**, then WebFetch for specific documentation pages. Cite URLs in Research Notes.

### Documenting Findings

The `## Research Notes` section is temporary scaffolding. It should contain:

- What was searched for and what was found (with file paths or URLs)
- Key existing patterns that the design should follow or deliberately deviate from
- Relevant constraints discovered (version requirements, API limits, etc.)
- Links to external documentation consulted

This section is removed before finalization — its contents get woven into the Design and Alternatives sections.

## Scope Determination

### Heuristics

For multi-scope repos, use these heuristics to determine the appropriate scope:

| Signal | Likely Scope |
|--------|-------------|
| Changes only files under one project | That project's scope |
| Modifies shared packages | Root scope |
| Adds a new CLI tool at repo root | Root scope |
| Changes the build system or CI | Root scope |
| Affects multiple project directories | Root scope |
| New feature within a single project | That project's scope |

### Cross-Cutting Concerns

Some changes affect a single project but have cross-cutting implications. In these cases:

- The RFC goes in the **primary project's scope**
- The RFC's Design section should document the cross-cutting impact
- Reference the root-scope design docs that need updating

## Drafting Guidance

### Problem vs. Proposal vs. Design

- **Problem:** _Why_ something needs to change. No solution language. A reader should agree the problem exists.
- **Proposal:** _What_ we're going to do about it, in 1-3 paragraphs. The elevator pitch. A reader should understand the approach without implementation details.
- **Design:** _How_ the proposal will be implemented. Code structure, data flow, configuration, migration steps. An implementer should be able to start work from this.

### Writing Genuine Alternatives

**The "real pros" test:** For each alternative, you must be able to name at least one genuine advantage — a reason a reasonable person might choose it over the proposed solution. If you can't, it's a strawman.

**Good alternative:**
> **Alternative: File-based caching** — Simpler to implement and debug. No additional service dependency. Works offline. However, doesn't share state across processes and has slower reads than in-memory solutions.

**Strawman (avoid):**
> **Alternative: File-based caching** — Too slow and doesn't support multi-process access.

### Open Questions vs. Resolved Questions

**Open Questions** in the RFC are genuine design decisions that haven't been made yet. They should:
- Be decisions, not research tasks (research should be done in Phase 2)
- Have identifiable trade-offs (not just "should we do X?")
- Not block the core proposal (if they do, go back to Phase 2)

Things that are **not** open questions (resolve these during authoring):
- "What library should we use?" → Resolve in Phase 2
- "Will this work?" → Verify in Phase 2
- "What does the user want?" → Resolve in Phase 1

Note: Open Questions in the document section are distinct from `> QUESTION:` markers. Markers are authoring-time action items that must be resolved before finalization. The Open Questions section lists decisions intentionally deferred beyond the document — to be resolved during implementation, research, or a follow-up. A `> QUESTION:` marker can be resolved by moving it into Open Questions when the user decides "not now."

### Writing Verification Plans

A good verification plan is **specific and actionable**. Someone should be able to follow it without interpretation.

**Good:**
> 1. Unit test: `CreateBackup()` returns error when ZFS snapshot fails (inject `zfs snapshot` error)
> 2. Integration test: full backup-restore cycle with the backup service's integration test suite
> 3. Manual test: trigger a network failure during backup, verify retry + notification

**Vague (avoid):**
> Test the backup system thoroughly. Make sure edge cases are covered.

## Inline Markers

Ordvik documents use inline markers for asynchronous feedback — the user edits the document directly to leave comments and questions, and the agent processes them on the next pass. This convention works across RFCs, design docs, and dispatch docs.

### Marker Types

Both marker types are visually distinct from the document body. A document under active editing has all open items visible in one scan — grep for `COMMENT:` or `QUESTION:` to find them.

COMMENT — User feedback on existing content. Written as **plain text** (no blockquote) because typing `>` prefixes is annoying. Format:

```markdown
COMMENT: This assumes we'll always have a single database, but the
multi-region RFC (0045) changes that. Need to account for replication lag.
```

QUESTION — Unresolved design decisions. The agent adds these as **blockquotes** during drafting to flag decisions that need user input. Format:

```markdown
> QUESTION: Should the cache TTL be configurable per-tenant, or a single
> global value? Per-tenant adds complexity but some tenants have very
> different access patterns.
```

The visual asymmetry is intentional: blockquoted QUESTIONs are agent-written and stand out from the prose; plain-text COMMENTs are quick for the user to type.

### Placement Rules

Both markers are always **full-line** — never inline on a line with other text. Place them on their own line immediately after the paragraph or section they refer to.

When commenting inside a list or nested structure, indent the marker to match the surrounding content so it doesn't break the list:

```markdown
- First item with some design detail
  COMMENT: This conflicts with the caching approach in section 3.
- Second item
```

```markdown
1. Deploy the service
   > QUESTION: Should we blue-green deploy or rolling update?
2. Verify health checks
```

### Processing Markers

**When the agent encounters COMMENTs** (plain text, user-written):

1. Read and understand the feedback in context.
2. Update the document to address the comment — change the text, add missing considerations, restructure the section, etc.
3. Remove the COMMENT marker after addressing it.
4. Summarize what changed in the conversation response so the user can verify.

If a COMMENT raises a fundamental issue that changes the direction (not just a local fix), flag it to the user before making sweeping changes.

**When processing QUESTIONs** (blockquoted, agent-written):

The user can respond to a QUESTION in two ways:
- **Edit the text directly** — if the surrounding text now answers the question, the agent removes the QUESTION marker and moves on.
- **Add a COMMENT responding to it** — the agent reads both, updates the text to reflect the decision, and removes both markers.

If the agent encounters an unresolved QUESTION with no user response, present it in conversation and ask.

**At finalization:** All COMMENT and QUESTION markers must be resolved and removed. A document with remaining markers is not ready for acceptance.

### Markers vs. Open Questions Section

`> QUESTION:` markers and the `## Open Questions` section serve different purposes:

- **Markers** are authoring-time escalations — the agent needs a decision from the user to continue writing the document. They are action items that block finalization. Every marker must be resolved and removed before the document is complete.
- **Open Questions** is a document section listing decisions that are intentionally deferred beyond the document's scope — things that will be resolved during implementation, in a research phase, or in a follow-up RFC. These persist in the final document as acknowledged unknowns.

A QUESTION marker may be resolved by moving it into the Open Questions section — that's a valid resolution when the user decides "we don't need to answer this now." The marker is removed; the question lives on as a documented deferral.

### When to Use Each

| Situation | Marker | Format | Who adds it |
|-----------|--------|--------|-------------|
| User disagrees with a design choice | COMMENT | plain text | User |
| User wants to add context the agent missed | COMMENT | plain text | User |
| User wants a section rewritten | COMMENT | plain text | User |
| Agent needs a decision to continue | QUESTION | blockquote | Agent |
| Agent identifies a trade-off with no clear winner | QUESTION | blockquote | Agent |
| Agent finds an inconsistency that needs user input | QUESTION | blockquote | Agent |

### Multi-Pass Editing

The COMMENT/QUESTION convention enables an asynchronous editing loop:

1. Agent writes a draft.
2. User reads the draft, adds COMMENT markers where they have feedback.
3. User tells the agent to process the comments (or the agent notices them on next read).
4. Agent addresses each comment, updates the document, removes resolved markers.
5. Repeat until no markers remain.

This is more efficient than conversational back-and-forth for detailed document feedback — the user can annotate the whole document at once rather than discussing one section at a time.

## Review Patterns

### Walking Through Open Questions

Present open questions one at a time or in small related groups. For each:

1. State the question clearly
2. Present the options with trade-offs
3. Recommend an approach if you have a basis for one
4. Update the Design section with the decision
5. Remove the resolved question

### Handling Conflicting Feedback

If the user's feedback contradicts an earlier decision:
1. Note the conflict explicitly
2. Re-present the original reasoning
3. Ask whether circumstances have changed or the original reasoning was wrong
4. Update both the decision and the rationale

### When Is the RFC "Done Enough"?

| Status | Bar |
|--------|-----|
| `draft` | Problem is clear, proposal exists, design has enough detail to discuss. Open questions are fine. |
| `considering` | All sections complete, open questions are minor or explicitly deferred. Design is reviewable by someone unfamiliar with the discussion. |
| `accepted` | No open questions. Design is implementable. Verification plan is actionable. Alternatives are genuinely evaluated. |

## Design Doc Integration

### Identifying Affected Design Docs

During Phase 3, check:

1. `docs/designs/` for root-scope docs
2. `<project>/docs/designs/` for project-scope docs
3. Look for docs that describe systems the RFC modifies
4. Consider whether the RFC introduces a new subsystem that needs its own design doc

### When to Update Design Docs

- **At RFC acceptance:** Update design docs to reflect the decided architecture. This is the primary trigger.
- **During implementation:** Refine design docs as implementation reveals details not covered in the RFC.
- **Not during RFC drafting:** The RFC is the proposal. Don't update design docs until the proposal is accepted.

### Keeping Design Docs Living

Design docs describe the _current_ state, not the _historical_ state. When updating:

- Replace outdated sections rather than appending history
- Reference the RFC that motivated the change (e.g., "Architecture updated per RFC 0042")
- Remove sections that no longer apply rather than marking them deprecated
