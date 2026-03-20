---
name: status
description: Create a session status report capturing completed work, in-progress items, and next steps. Use when the user asks for a status update or wants to log progress.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git log *)
  - Bash(git diff *)
  - Bash(git status *)
  - Bash(git branch *)
  - Bash(date *)
  - Bash(python *)
  - AskUserQuestion
---

# Status Report Skill

Creates a timestamped status report capturing work done in the current session. Output routing determines where it goes — local markdown by default, Obsidian if configured.

## Parsing Invocation

The user invokes this skill as `/ordvik:status [title]`.

- With a title: use it as the report title
- Without: derive from the session's work (e.g., "RFC authoring and dispatch setup")

## Workflow

### Step 1: Gather Context

Review the current session's work:

1. Recent git commits: `git log --oneline -20`
2. Current branch and status: `git status`
3. Uncommitted changes: `git diff --stat`
4. Conversation history for context on what was discussed and decided

### Step 2: Generate the Status Entry

Format:

```markdown
## HH:MM - [Brief Title]

### Completed
- Bullet points of completed work
- Include commit hashes for significant changes

### In Progress
- Current work items not yet finished

### Next Steps
- Planned next actions

### Notes
- Relevant observations, blockers, or decisions made
- Context that would be useful when resuming this work
```

Keep entries concise but informative. Emphasize decisions and outcomes over process.

### Step 3: Route Output

Determine output destination by checking (in order):

1. `.ordvik.yml` → `output.mode` field
2. `ORDVIK_OUTPUT` environment variable
3. Default: `local`

**Local mode** (default):
- Write to `docs/status/YYYY-MM-DD-status.md`
- If the file exists, append the new entry with a blank line separator
- Create `docs/status/` if it doesn't exist

**Obsidian mode** (`ORDVIK_OUTPUT=obsidian`):
- Use the output helper script: `python ${CLAUDE_PLUGIN_ROOT}/scripts/output.py`
- Falls back to local mode if Obsidian is unavailable
- Path: `projects/{project-name}/YYYY-MM-DD-{project-name}-status`

**Both mode** (`ORDVIK_OUTPUT=both`):
- Write locally AND push to Obsidian

### Step 4: Report

Confirm where the status was written and show a brief summary.
