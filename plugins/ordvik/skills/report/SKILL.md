---
name: report
description: Create a topical research or investigation report. Use when the user asks for a report on a specific topic, technology, or investigation.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git log *)
  - Bash(git diff *)
  - Bash(git status *)
  - Bash(python *)
  - WebSearch
  - WebFetch
  - Task
  - AskUserQuestion
---

# Report Skill

Creates a topical report — a structured document on a specific subject, typically the result of research or investigation. Output routing determines where it goes.

## Parsing Invocation

The user invokes this skill as `/ordvik:report <topic> [options]`.

| Argument | Description |
|----------|-------------|
| `<topic>` | What the report is about (e.g., "nats migration options", "test coverage gaps") |

If the user asks conversationally to "write a report on X" or "investigate X and write it up", treat as invoking this skill.

## Workflow

### Step 1: Scope the Report

Use `AskUserQuestion` to clarify:

- What specific questions should the report answer?
- What's the intended audience? (for the user's own reference, for a team, for a decision-maker)
- How deep should the investigation go?
- Are there specific areas of the codebase to focus on?

### Step 2: Research

Use appropriate tools based on the topic:

- **Codebase investigation**: Grep, Glob, Read, Task/Explore agents
- **External research**: WebSearch, WebFetch for documentation, comparisons, best practices
- **Git history**: git log, git diff for understanding how things changed over time

### Step 3: Write the Report

Structure depends on the topic, but common sections:

```markdown
# Report: [Topic]

**Date**: YYYY-MM-DD
**Author**: [agent, on behalf of user]

## Summary

2-3 sentence executive summary of findings.

## Background

Context needed to understand the findings.

## Findings

The core content — organized by theme, question, or area.

## Recommendations

Actionable suggestions based on the findings.

## References

Links, file paths, and sources consulted.
```

Adapt the structure to fit the topic. A technology comparison might have a comparison table. A code investigation might have file paths and code snippets. A gap analysis might have a prioritized list.

### Step 4: Route Output

Same routing as the status skill:

1. `.ordvik.yml` → `output.mode` field
2. `ORDVIK_OUTPUT` environment variable
3. Default: `local`

**Local mode** (default):
- Write to `docs/reports/YYYY-MM-DD-<topic-slug>.md`
- Create `docs/reports/` if it doesn't exist

**Obsidian mode**:
- Use the output helper script
- Path: `reports/YYYY-MM-DD-<topic-slug>`

**Both mode**: Write locally AND push to Obsidian.

### Step 5: Report

Show where the report was written and a brief summary of key findings.
