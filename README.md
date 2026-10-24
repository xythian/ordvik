# ordvik

Agent toolkit for structured software development — planning model, RFC management, dispatch documents, and reporting, packaged as a Claude Code plugin.

## What is ordvik?

Ordvik packages a set of practices for AI-assisted software development as a distributable toolkit:

- **RFC management** — Lifecycle management for design proposals with a 5-phase collaborative authoring workflow
- **Dispatch documents** — Structured work definitions that break planned work into agent-executable tasks with validation
- **Development process** — One written path from goal to merged PR: when to research, write an RFC, plan and dispatch, what to ask the operator, how PRs merge (`/ordvik:process`)
- **Planning model** — Thread registry, backlog, dispatch plans with a conductor, the operator asks queue, and planning sessions
- **Reporting** — Status reports and research documents with configurable output routing

Skills work in layers: RFC management and reporting are always available. The planning layer (dispatch, threads, plans) activates when a repo adopts the planning structure. An init skill bootstraps everything.

## Installation

```bash
# Add the repo as a marketplace source
claude plugin marketplace add xythian/ordvik

# Install the plugin
claude plugin install ordvik
```

Skills are invoked as `/ordvik:rfc`, `/ordvik:dispatch`, `/ordvik:status`, etc.

## Skills

| Skill | Description | Layer |
|-------|-------------|-------|
| `/ordvik:init` | Bootstrap planning structure in a repo | Bootstrap |
| `/ordvik:process` | The development process from goal to merged PR | Core |
| `/ordvik:rfc` | RFC lifecycle — create, author, list, review, status updates | Core |
| `/ordvik:dispatch` | Dispatch document creation, listing, and review | Planning |
| `/ordvik:status` | Session status reports | Core |
| `/ordvik:report` | Research/topical reports | Core |
| `/ordvik:planning` | Planning sessions — review threads, triage backlog, close plans | Planning |
| `/ordvik:verify` | Review completed work for a thread or a plan | Planning |
| `/ordvik:garden` | Gardening dispatch templates for codebase surveys | Planning |
| `/ordvik:asks` | Everything waiting on the operator: decide, review, do; recurring steps to automate | Planning |
| `/ordvik:research` | Conduct and record structured research — standalone docs or RFC appendices | Core |
| `/ordvik:name` | Generate names by combining stems from real languages | Utility |

## Repo Models

Ordvik supports two repo models as first-class patterns, configured via `.ordvik.yml`:

**Single-scope** — One RFC namespace, one codebase. The default for most repos.

**Multi-scope** — For monorepos where subprojects have independent RFC histories, design docs, and source paths. Each scope is a named project unit.

### Cross-Repo Dispatch

A repo can serve as a **planning hub** — hosting the backlog, thread registry, and dispatch docs — while dispatching work to **other repos**. This is common when a monorepo plans work that targets satellite repos. The dispatch doc's `repo` field declares the target, clone URL, and what reference access the agent needs back to the planning hub.

## Conventions

### Inline Markers

Documents under active editing use two marker types for asynchronous feedback:

- `COMMENT:` — Plain text, written by the user. Feedback on existing content.
- `> QUESTION:` — Blockquoted, written by the agent. Flags decisions that need user input.

Both are full-line, never inline. Both must be resolved and removed before a document is finalized. An `## Open Questions` section in the document is different — those are intentionally deferred decisions that persist in the final document.

### Convention File Discovery

Skills check for project-local convention files at well-known paths, falling back to the plugin's built-in defaults. Projects customize by creating files at the expected paths — no forking required.

## Name

From Latin *ordo* (order, arrangement) and Old Norse *vik* (inlet, harbor) — "ordered harbor," a structured place where work is organized and dispatched from.

The name follows a pattern used across the family of projects: compound words formed from stems across languages, aiming for unique, short, and evocative.
