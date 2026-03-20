---
name: research
description: Conduct and record structured research on a topic — codebase investigation, technology evaluation, ecosystem survey, or design space exploration. Produces a standalone research document or an RFC appendix.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git log *)
  - Bash(git diff *)
  - Bash(git status *)
  - WebSearch
  - WebFetch
  - Task
  - AskUserQuestion
---

# Research Skill

Conducts structured research and records it as a durable artifact. Research documents capture investigation, evaluation, and analysis — they inform decisions but don't propose work the way RFCs do.

## Output Modes

Research can land in two places, depending on context:

1. **Standalone document** — `docs/research/<slug>.md` (or `<scope>/docs/research/` in multi-scope repos). Use when the research is broadly useful, covers a topic area, or may feed into multiple future RFCs.

2. **RFC appendix** — `## Appendix N: Research — <Topic>` appended to an existing RFC. Use when the research was conducted in service of a specific RFC and is most useful as context alongside the proposal. This is a judgment call made during the session.

Both formats are valid. The choice depends on whether the research has a life independent of any particular RFC.

## Parsing Invocation

The user invokes as `/ordvik:research <topic> [options]`.

| Option | Description |
|--------|-------------|
| `<topic>` | What to research |
| `--rfc <number>` | Attach as an appendix to this RFC instead of standalone |
| `--scope <project>` | Which scope's research directory (multi-scope repos) |

If the user asks conversationally to "research X", "investigate X", "evaluate X and write it up", or "look into X", treat as invoking this skill.

## Standalone Research Document

### Step 1: Scope the Research

Use `AskUserQuestion` to clarify:

- What specific questions should this research answer?
- How deep should the investigation go?
- Are there specific areas to focus on or exclude?
- Is there a decision this research needs to inform?

### Step 2: Conduct Research

Use appropriate tools based on the topic:

**Codebase investigation:**
- Grep, Glob, Read for finding patterns and understanding structure
- Task/Explore agents for broad investigations requiring many queries
- git log for understanding how things evolved

**Technology evaluation:**
- WebSearch for current documentation, comparisons, community health
- WebFetch for specific documentation pages
- Always verify against current sources — never rely on built-in knowledge for version-specific claims

**Ecosystem survey:**
- WebSearch for landscape overview
- WebFetch for each candidate's documentation
- Look for: maturity, maintenance activity, community size, known issues, integration patterns

### Step 3: Write the Document

File location: `docs/research/<topic-slug>.md` (or `<scope>/docs/research/` in multi-scope repos). Create the directory if it doesn't exist.

Filename: descriptive slug, no numeric prefix. Examples:
- `nats-jetstream-evaluation.md`
- `arm64-cluster-hardware-options.md`
- `source-code-search-tools.md`

Structure:

```markdown
# Research: [Topic Name]

**Date**: YYYY-MM-DD
**Author**: [user] + Claude
**Context**: One-sentence problem statement and scope.
**Related**: [Optional — RFCs, other research docs, design docs]

## Executive Summary

2-3 paragraphs synthesizing key findings and recommendations.
This is the most important section — a reader should be able to
make a decision from this alone.

Explicit verdicts: "Bottom line: ..."

## [Analysis Sections]

Organized by theme, candidate, or question. Each section should have:
- What was investigated
- What was found (with evidence and sources)
- Trade-offs and constraints
- Verdict or assessment

## Recommendations

Actionable next steps based on findings. Reference specific
candidates, approaches, or decisions.

## Sources

Links and references consulted, with brief annotations.
```

Key conventions:
- **Executive summary is mandatory** and must contain explicit recommendations. A reader should be able to act on the executive summary alone.
- **Verdicts are explicit** — don't leave conclusions implicit. "Bottom line: X is the best fit because Y" is better than a neutral comparison that forces the reader to draw their own conclusion.
- **Sources are cited inline and collected at the end.** Include URLs. Note access dates for things that change (API docs, pricing pages).
- **Author format**: `[human] + Claude` — these are collaborative investigations.

### Step 4: Report

Show where the document was written and highlight key findings.

## RFC Appendix Mode

When research is conducted in service of a specific RFC (`--rfc <number>` or during an RFC authoring session):

### Judgment Call: Standalone vs. Appendix

Ask (or use judgment):
- **Standalone** if: the research covers a broad topic area, could inform future RFCs, or has value independent of this RFC
- **Appendix** if: the research is tightly scoped to this RFC's design decisions, wouldn't make sense without the RFC's context, or is primarily detailed evidence supporting the RFC's choices

It's fine to do both — a standalone research doc for the broad investigation, and an appendix summarizing what's directly relevant to the RFC.

### Appendix Format

Append to the RFC, after the Verification Plan (and after any existing appendices):

```markdown
## Appendix A: Research — [Topic]

This appendix captures [what was researched and why it matters to this RFC].

### A.1 [Subtopic]

[Detailed findings organized by subtopic]

### A.2 [Subtopic]

[...]
```

Conventions:
- **Lettered appendices**: Appendix A, B, C, etc.
- **Numbered subsections within**: A.1, A.2, B.1, etc.
- **Title format**: `## Appendix X: Research — [Topic]` when it's research. Other appendices (reference material, code patterns, evidence) use `## Appendix X: [Title]` without the "Research —" prefix.
- **Opening line** explains what was researched and why it's relevant to this RFC.
- **Key insights for the RFC** should be called out explicitly — don't make the reader hunt for why this appendix matters.
- Multiple research appendices are common — RFC 0047 has five (agents survey, protocol research, context management, skills architecture, protocol reference).

### Relationship to RFC Authoring

During `/ordvik:rfc author`, Phase 2 (Solution Research) produces findings. At the end of Phase 2:

1. Findings that directly shaped the design → woven into the Design section
2. Findings that provide useful background/evidence → appendix candidates
3. Broad research that stands alone → standalone research document, referenced from the RFC's `**Related**` field

The temporary `## Research Notes` section from Phase 2 is the staging area. At finalization, its contents are distributed across the design text, appendices, and/or standalone documents. Nothing should remain in Research Notes.

## Scope in Multi-Scope Repos

In multi-scope repos, research documents follow the same scoping as RFCs:

- Root scope: `docs/research/` — cross-cutting research
- Project scope: `<scope>/docs/research/` — project-specific research

Use `--scope` to specify, or infer from context. Cross-cutting technology evaluations go in root scope. Project-specific investigations go in the project scope.
