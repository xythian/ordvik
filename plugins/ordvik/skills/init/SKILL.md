---
name: init
description: Bootstrap ordvik planning structure in a repo. Creates .ordvik.yml, docs/rfcs/, docs/planning/, and convention files. Safe to run on already-initialized repos.
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - Bash(git config *)
  - Bash(git remote *)
  - AskUserQuestion
---

# Init Skill

Bootstraps a repo for ordvik. Creates the planning structure, scope registry, and convention files. Idempotent — running on an already-initialized repo skips existing files and reports what's already set up.

## Parsing Invocation

The user invokes this skill as `/ordvik:init [options]`.

| Option | Description |
|--------|-------------|
| (none) | Interactive setup — detect, ask, create |
| `--dry-run` | Show what would be created without writing anything |
| `--upgrade` | Check for updates to plugin-managed files (future — not yet implemented) |

## Workflow

### Step 1: Detect Existing Structure

Check for each of these and report what already exists:

- `.ordvik.yml` — scope registry and config
- `docs/rfcs/` — RFC directory
- `docs/rfcs/conventions.md` — project-local RFC conventions
- `docs/research/` — research documents
- `docs/planning/` — planning structure
- `docs/planning/threads.md` — thread registry
- `docs/planning/backlog.md` — backlog
- `docs/planning/dispatch/` — dispatch doc directory
- `CLAUDE.md` or `AGENTS.md` — agent guidance

Report a summary: "Found: X, Y, Z. Missing: A, B, C."

If everything exists including `.ordvik.yml`, report "This repo is already initialized for ordvik" and offer to review the configuration.

### Step 2: Determine Repo Model

Ask the user (via `AskUserQuestion`):

**"Is this a single-project repo or a monorepo with multiple subprojects?"**

- **Single-scope**: One codebase, one RFC namespace. Simplest setup.
- **Multi-scope**: Subprojects with independent RFC histories and design docs.

If `docs/rfcs/` already exists with files, examine the directory structure to infer the model:
- If RFC files exist only in `docs/rfcs/` → suggest single-scope
- If RFC files exist in `<project>/docs/rfcs/` directories → suggest multi-scope, list discovered scopes

### Step 3: Gather Project Info

Use `AskUserQuestion` and `git config` / `git remote` to collect:

- **Project name**: infer from repo directory name or git remote
- **Author default**: from `git config user.name`
- **Scopes** (multi-scope only): ask the user to list subprojects, or confirm discovered scopes. For each scope, gather:
  - Name (short identifier, e.g., `search`, `frontend`)
  - RFC directory path (default: `<name>/docs/rfcs/`)
  - Design docs path (default: `<name>/docs/designs/`)
  - Source paths (which directories it owns)
  - Language (optional)

### Step 4: Create .ordvik.yml

Write `.ordvik.yml` at the repo root. Skip if it already exists.

**Single-scope template:**

```yaml
ordvik:
  initialized_version: 0.2.0
  model: single-scope

scopes:
  root:
    rfcs: docs/rfcs/
    designs: docs/designs/
    research: docs/research/
```

**Multi-scope template:**

```yaml
ordvik:
  initialized_version: 0.2.0
  model: multi-scope

scopes:
  root:
    rfcs: docs/rfcs/
    designs: docs/designs/
    research: docs/research/
    description: Cross-cutting concerns and shared infrastructure
  # One entry per subproject, filled from Step 3
```

### Step 5: Create Directory Structure

For each item below, create only if it doesn't already exist:

**Always created:**
- `docs/rfcs/` directory
- `docs/research/` directory

**Planning layer** — ask: "Do you want to set up the planning structure (threads, backlog, dispatch docs)?" Default yes.
- `docs/planning/` directory
- `docs/planning/dispatch/` directory
- `docs/planning/threads.md` from template (see below)
- `docs/planning/backlog.md` from template (see below)

**Multi-scope only:**
- `<scope>/docs/rfcs/` for each declared scope
- `<scope>/docs/designs/` for each declared scope (if the directory's parent exists)
- `<scope>/docs/research/` for each declared scope (if the directory's parent exists)

### Step 6: Create Convention Files

Skip any file that already exists.

**RFC conventions** — write `docs/rfcs/conventions.md` only if the repo is multi-scope or has special numbering needs. Single-scope repos use the plugin default and don't need a project-local file.

For multi-scope repos, generate from the scope registry:

```markdown
# RFC Conventions — {{PROJECT_NAME}}

## Project-Scoped RFC Structure

### Root Scope

- Directory: `docs/rfcs/`
- For cross-cutting concerns and shared infrastructure

### Project Scopes

| Project | RFC Directory | Notes |
|---------|--------------|-------|
{{for each scope}}
| {{name}} | `{{rfcs}}` | {{description or blank}} |
{{end}}
```

### Step 7: Offer Agent Guidance

Ask: "Do you want to add agent guidance to CLAUDE.md (or AGENTS.md)?"

If yes, read `${CLAUDE_PLUGIN_ROOT}/templates/AGENTS-guidance.md` and append it to the repo's `CLAUDE.md` or `AGENTS.md` (whichever exists; prefer `CLAUDE.md`; create `CLAUDE.md` if neither exists). This guidance covers:

- **Development process** — a summary of `/ordvik:process` and a pointer to it. Afterwards, tell the user to add the repo's own bindings next to it: default branch, test commands, and which hosts serve as each venue in the process's "Where work runs" table.
- **Codebase exploration** — use built-in tools (Glob, Grep, Read, Edit, Write) instead of shell commands. This avoids per-command approval prompts and produces better results.
- **Subagent communication** — return results as text, don't write temp files.
- **Build & test** — use project scripts, not bare language commands.

If the file already contains a "Codebase Exploration" section, skip — don't duplicate.

### Step 8: Offer Worktree Script

Ask: "Do you want to install the `wt` worktree manager script?"

If yes, copy `${CLAUDE_PLUGIN_ROOT}/scripts/wt` to `scripts/wt` in the repo and make it executable. The `wt` script manages git worktrees for parallel agent sessions — creating worktrees from dispatch docs, launching agents in tmux panes, and cleaning up merged branches.

Skip if `scripts/wt` already exists. Note the `pyyaml` dependency (needed for parsing dispatch doc frontmatter).

In the same step, offer `scripts/plan` (copy `${CLAUDE_PLUGIN_ROOT}/scripts/plan`, make it executable): it reads a dispatch plan in `docs/planning/plans/` and says which dispatch docs can launch now. Also offer `scripts/asks` (same way; it imports `scripts/plan`): it lists everything waiting on the operator (see `/ordvik:asks`). With it, create `docs/planning/asks/` and `docs/planning/asks/archive/` (with a `.gitkeep`). Skip any that already exist.

If the repo has RFCs and is on GitHub, also offer RFC number reservation: copy `${CLAUDE_PLUGIN_ROOT}/scripts/rfc` to `scripts/rfc` (executable) and `${CLAUDE_PLUGIN_ROOT}/templates/workflows/rfc-reservation.yml` to `.github/workflows/rfc-reservation.yml`, and suggest adding `python3 scripts/rfc check` to the test suite. Set the workflow's `workflow_run.workflows` to the names of every workflow behind a required status check (the template says `ci`): it tries the merge as each finishes, and the last one's run succeeds. It merges with a token from a dedicated GitHub App, so a ruleset that restricts updates or requires approval can let only that App through. Walk the user through: create a private GitHub App with Contents and Pull requests read and write and no webhook, install it on this repo only, add it to the default branch ruleset's bypass list ("For pull requests only"), and create an environment `rfc-reservation` limited to the default branch holding variable `RFC_APP_ID` and secret `RFC_APP_PRIVATE_KEY`. Skip each file that already exists.

### Step 9: Report and Suggest Next Steps

Summarize what was created, then suggest:

1. **Review `.ordvik.yml`** — check that scopes and paths are correct
2. **Create your first RFC** — `/ordvik:rfc create <title>`
3. **Create a dispatch doc** — `/ordvik:dispatch create <slug>` (if planning layer was set up)
4. **Commit the scaffolding** — offer to commit with an appropriate message

---

## Templates

Template files are in `${CLAUDE_PLUGIN_ROOT}/templates/`:

| Template | Target Path | Description |
|----------|-------------|-------------|
| `threads.md` | `docs/planning/threads.md` | Thread registry with status definitions |
| `backlog.md` | `docs/planning/backlog.md` | Prioritized backlog (P0-P2 + Done) |
| `AGENTS-guidance.md` | Appended to `CLAUDE.md` | Development process summary, codebase exploration and build/test guidance |
| `workflows/rfc-reservation.yml` | `.github/workflows/rfc-reservation.yml` | Merges RFC number reservation PRs once CI passes on them |

Scripts in `${CLAUDE_PLUGIN_ROOT}/scripts/`:

| Script | Target Path | Description |
|--------|-------------|-------------|
| `wt` | `scripts/wt` | Worktree manager — create, launch, list, clean worktrees |
| `plan` | `scripts/plan` | Dispatch plan status — what can launch, what needs a doc, which gates to ask |
| `rfc` | `scripts/rfc` | RFC number reservation, next number, and the index check |
| `asks` | `scripts/asks` | Operator asks — every decision, review and step waiting on the human |

Read each template file and write it to the target path. Do not modify the templates — they're starter content that the user will customize.

---

## Idempotency Rules

- **Never overwrite** existing files. If a file exists, skip it and report "already exists."
- **Never modify** `.ordvik.yml` if it exists — report the current config and suggest manual edits if something looks wrong.
- **Directories** are always safe to create (mkdir -p semantics).
- Running init twice produces the same result as running it once — no side effects on the second run.

## Versioning and Upgrades

The `ordvik.initialized_version` field in `.ordvik.yml` records which version of ordvik initialized the repo. This lets future upgrade logic know what conventions the repo was set up with.

**v0.1 upgrade path:**
1. `claude plugin update ordvik` — updates plugin skills automatically (no repo changes needed)
2. Plugin-managed repo files (`scripts/wt`, `scripts/plan`, `scripts/rfc`, `scripts/asks`, the reservation workflow, agent guidance) must be updated manually for now

**Future `--upgrade` flag** (not yet implemented):
- Compare `initialized_version` against the current plugin version
- Offer to update plugin-managed files (wt script, agent guidance) — show the diff, ask for confirmation
- Run migrations if conventions changed between versions (e.g., new frontmatter fields)
- Update `initialized_version` after successful upgrade
- Convention files owned by the project (`.ordvik.yml` scopes, `docs/rfcs/conventions.md`) are never modified by upgrade — those belong to the project

## Adoption in Existing Repos

For repos that already have planning artifacts:

- Init detects the existing structure and skips most creation steps
- The main value is generating `.ordvik.yml` — the scope registry that other skills read
- Existing project-local skills (`.claude/skills/rfc/`) take precedence over ordvik plugin skills
- The migration path is gradual: use ordvik skills alongside existing ones, retire the old ones when ready
