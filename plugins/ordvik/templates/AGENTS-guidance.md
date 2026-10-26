## Development Process

This repo follows ordvik's development process: goal, research, RFC, plan and dispatches, implement, PR, close. `/ordvik:process` has the whole of it, including when a change needs an RFC, what to ask the operator, how to drive a PR, and the flat-history rule. In short:

- Ask the operator only to decide, review, approve a dry run, or do what needs their own hands. A list of commands for them to type is a missing command.
- Merging an RFC's PR accepts it, unless the PR says the RFC is tabled.
- New dispatch docs are named `YYYYMMDD-<slug>.md`; the name is the thread id.
- Rebase onto the default branch, no merge commits; tidy your own branch's commits before review without asking.
- A dispatch closes in its own PR. With a planning repository, link each PR to its dispatch (`ordvik planning link <project>/<id> --pr <owner/name#n> --task <n>` or `--done`) and keep the dispatch, plan and planning repository out of commits, branches and PR bodies. Large threads write a short retro; hard-won fixes get a TIL.

## Planning & Backlog

This repo uses ordvik for planning. Key files live in `<planning>/`.

**Where planning lives.** `<planning>` below is the project's planning directory: `projects/<project>/` in the planning repository when `.ordvik.yml` names one (`planning: {repo: <owner>/<planning-repo>, project: <project>}`; cloned beside the code checkout as `planning/`, or at `$ORDVIK_PLANNING`), else `docs/planning/` in the repository. In the planning repository the backlog is one file per item in `backlog/` (ordered by `priority`, then `rank`) and threads are files in `threads/`, where `docs/planning/` has `backlog.md` and `threads.md`; status reports are in `status/` (else `docs/status/`); dispatch docs carry `id:`, `thread:` and `repo:` (else the id is `thread:`); plans list `dispatches:` (else `threads:`); and a change is a commit straight to its `main` (fetch, change, `scripts/plan check`, commit, push), not a PR. `scripts/plan` and `scripts/asks` read either layout (in the planning repository they run `ordvik planning`).

- **Backlog**: `<planning>/backlog/` (or `backlog.md`) — prioritized work items
- **Threads**: `<planning>/threads/` (or `threads.md`) — work threads with status
- **Dispatch docs**: `<planning>/dispatch/` — agent-executable work packages
- **Operator asks**: `scripts/asks status` lists everything waiting on the operator. A human gate belongs in its dispatch doc, and a plan gate belongs in its plan. Only an ask with no other home gets a file in `<planning>/asks/`. Mark a gate's task `— DONE` once the operator has done it.

When the user says "add to backlog", "backlog this", or similar — they mean add an item to `<planning>/backlog/` (or `backlog.md`), not save a memory or note. Read the file first to match the existing format and priority section.

## Codebase Exploration

When searching or exploring the codebase, prefer built-in tools over shell commands:

- **Find files**: Use `Glob` (not `find` or `ls -R`)
- **Search content**: Use `Grep` (not `grep`, `rg`, `awk`)
- **Read files**: Use `Read` (not `cat`, `head`, `tail`, `sed`)
- **Edit files**: Use `Edit` (not `sed -i`, `awk`, `perl -pi`)
- **Write files**: Use `Write` (not `echo >`, `cat <<EOF >`)
- **Complex exploration**: Use the `Agent` tool with `subagent_type=Explore`

Only fall back to shell commands when you need something these tools genuinely can't do (e.g., `wc -l`, `file`, `stat`, process-related commands). Built-in tools are faster, safer, and don't require per-command approval.

## Subagent Communication

Subagents return results as text directly to the parent agent via the Agent tool's return value. Do NOT write files to `/tmp/` or anywhere else to pass data between agents. Instead, include all relevant findings in your text output — that IS the result the parent receives.

## Build & Test

Always use project build/test scripts rather than bare language commands:

```bash
# Do this:
./scripts/test.sh
./scripts/build.sh

# Not this:
go test ./...
cargo build
npm run build
```

The scripts handle build tags, cross-compilation, test fixtures, and other project-specific concerns. Bare commands may miss build constraints or produce incorrect results.

For test tiers:
- **Fast tier** (default): `./scripts/test.sh` — use during iterative development
- **Full tier**: `./scripts/test.sh full` — use before declaring work complete
- **Integration tests**: `./scripts/test.sh list` to discover available suites
