# Dispatch Conventions — Default

Plugin default conventions. Projects customize by creating `docs/planning/conventions.md` which takes precedence over this file.

## Status Lifecycle

Dispatch documents track their lifecycle via a `status` frontmatter field:

```
active  →  done
        →  superseded
```

- **`active`** — Work is in progress or pending. This is the default; the field can be omitted and missing `status` is treated as `active`.
- **`done`** — All tasks completed, PR merged, completion criteria verified. Set via the `close` operation.
- **`superseded`** — Replaced by a newer dispatch doc. Set via `close --superseded-by <ref>`.

When superseding a dispatch doc, two fields are added:

```yaml
status: superseded
superseded_by: "WWW-TTT-new-slug"
```

And a blockquote is inserted immediately after the title heading:

```markdown
# Thread TTT: Original Title

> **Superseded** by [WWW-TTT-new-slug](./WWW-TTT-new-slug.md). This document is retained for historical context.
```

## Task Completion Markers

Tasks are marked complete by appending ` — DONE` to the task heading:

```markdown
## Task 1: Implement the widget — DONE
```

Rules:
- Use an em dash (`—`), not a hyphen or en dash
- No emoji — `— DONE` is the standard marker
- The marker is appended to the existing heading text, not placed on a new line
- Use the `mark-done` operation to apply the marker consistently

## Task Type Markers

Task headings can include a parenthetical type marker after the task number:

| Marker | Meaning |
|--------|---------|
| *(none)* | Standard implementation task |
| `(Human Gate)` | Requires human execution |
| `(Audit)` | Verification task; benefits from fresh agent context |
| `(Contract)` | Defines immutable tests/criteria for subsequent tasks |

Markers are part of the heading text and survive the `— DONE` append:

```markdown
## Task 5 (Audit): Verify data pipeline end-to-end — DONE
```

The `human_gates` frontmatter field lists Human Gate tasks. Audit and Contract tasks do not have a corresponding frontmatter field — they are identified by heading markers only.

## Wave Grouping

Waves are batches of concurrent dispatch threads. Conventions:

- **3-digit zero-padded numbers** — `001`, `002`, `003`
- **Concurrent independent threads** — Threads within a wave can execute in parallel
- **Standalone threads omit wave** — If work isn't part of a batch, omit the wave prefix from both the filename and frontmatter
- **Wave numbering is global** to the project, not per-thread

Filename pattern:
- With wave: `WWW-TTT-slug.md` (e.g., `003-015-add-auth.md`)
- Without wave: `TTT-slug.md` (e.g., `015-add-auth.md`)

## Retro Format

Wave retrospectives use the template defined in the planning skill (`/ordvik:planning wave-close`). That template is the single source of truth for retro structure.

Key points:
- Retros live in `docs/planning/dispatch/` while the wave is active (e.g., `wave-003-retro.md`)
- After wave close-out, retros are archived along with all wave dispatch docs
- Retros are written as part of wave close-out, not thread close-out

## Close-Out Checklist

### Thread-Level Close-Out

Before closing a dispatch thread (`/ordvik:dispatch close <file>`):

- [ ] All tasks marked `— DONE`
- [ ] PR merged (or work landed on target branch)
- [ ] `status` set to `done` (or `superseded` with `superseded_by`)
- [ ] All completion criteria checked off
- [ ] Backlog updated — completed items marked done, follow-on items added if needed
- [ ] Pre-existing issues captured for gardening
- [ ] Audit tasks (if any) were executed and passed
- [ ] Contract tests (if any) still pass without modification to contract files

### Wave-Level Close-Out

Before closing a wave (`/ordvik:planning wave-close <N>`):

- [ ] All threads in the wave closed (status `done` or `superseded`)
- [ ] Wave retro written
- [ ] All wave dispatch docs + retro archived to `archive/`
- [ ] Backlog updated with follow-on items from retro
- [ ] Thread registry updated if thread statuses changed

## Archive Convention

Completed dispatch docs are archived to preserve the dispatch folder as a clean work queue.

**Archive location**: `docs/planning/dispatch/archive/`

**Rules**:
- Files keep their original names when archived
- `done` threads are archived after the wave retro is written (so they're available during retro)
- `superseded` threads are archived immediately (no need to wait for retro)
- The `list` operation excludes `archive/` by default — use `--archived` to include them
- Standalone (non-wave) threads that are closed as `done` archive immediately since there's no wave retro to wait for

## Folder as Work Queue

The `docs/planning/dispatch/` directory should contain only **active work**:

- **Active dispatch docs** — threads with work in progress (`status: active` or missing)
- **Pending dispatch docs** — threads not yet started but planned for the current or next wave
- **Current-wave retros** — retros for waves that haven't completed close-out yet

Everything else belongs in `archive/`. This convention makes the dispatch folder a scannable work queue — if it's in the folder, it needs attention.
