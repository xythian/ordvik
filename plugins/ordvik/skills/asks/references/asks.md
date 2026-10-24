# Operator asks

Much of the work here waits on one person. An RFC needs accepting, a PR
needs merging, a tailnet grant needs adding, a restore drill needs running
in production, a decision card sits unanswered in a project thread. Each of
these is recorded somewhere, but in a dozen different places. When a lot
is in flight, the operator has no single answer to "what is waiting on me?",
and an ask buried in a thread is easy to miss.

This doc defines an **ask**, the **status tool** that collects asks from
where they already live (`scripts/asks`), the **sweep** that adds what git
cannot see, and how recurring asks turn into automation.

## Principles

1. **Read asks where they live.** A human gate belongs in its dispatch doc,
   a plan gate in its plan, an RFC decision in the RFC. The tool reads
   those. Nobody copies them into a second list that can drift.
2. **A file only for an orphan.** An ask with no doc to live in gets one
   small file of its own, so two threads adding asks never touch the same
   file.
3. **Every ask ends with an answer.** It is done, decided, rejected,
   deferred to a date, or sent to be automated. An ask never just goes
   quiet.
4. **An ask the operator keeps getting is a missing command.** A human
   gate is for what only a person can do; a list of commands to type is a
   command nobody has written yet. Kinds of steps that keep coming back
   are counted and become backlog items.

## What an ask is

An ask is something only the operator can move. It has a **kind**, which
is the verb. Production steps made of existing commands are `approve`, not
`do`: the operator approves the dry run and an agent runs it.

| Kind | Examples |
|---|---|
| `decide` | Accept or reject an RFC; answer a decision card; pass a plan gate; choose between next steps |
| `review` | Review and merge a PR; check an app on a phone or iPad; verify a production result |
| `approve` | Approve a dry run (a deploy, a migration, a converge) that an agent then runs with `--commit` where the operator's credentials are, for example a Remote Control session on their machine |
| `do` | Steps that need the operator's own hands, devices or accounts: a policy edit in a console, a credential in a password manager, a check on a phone, files only they have |

It also has **categories** for the kind of step it is: `tailnet-policy`,
`restore-drill`, `production-rollout`, `device-check`, `credentials`,
`rfc`, or any other word an ask file names. Categories are how recurring
work becomes visible.

## Where asks come from

| Source | An ask when | Read from |
|---|---|---|
| Dispatch docs | An entry in `human_gates` whose `## Task N` heading is not marked `— DONE`, in an unarchived doc that is not `done` or `superseded` | The ref (`origin/master`) |
| Plans | A gate without `passed:` that an unfinished thread waits on | The working tree |
| RFCs | Status `considering`, under any `docs/rfcs/` in the repo | The ref |
| Ask files | A file in `docs/planning/asks/` with no `answered:` | The working tree |
| Open PRs | Waiting on the operator's review or merge | The sweep (GitHub) |
| Agent threads | Blocked on the operator, or holding an open decision card | The sweep |

Draft RFCs are not asks; they are still being written. The tool counts
them and lists those untouched for 60 days (`--all`), because a long-idle
draft is a triage question: revive it, park it, or reject it.

A human gate whose earlier tasks are not marked `— DONE` is listed with a
`*`. Either the gate is not reachable yet, or the doc is stale because the
work shipped and nobody marked the tasks or closed the dispatch. Both cases
need someone to look, so the gate stays on the list.

### Ask files

An ask with no doc to live in, such as files only the operator has, a
choice recorded only in a backlog note, or a grant a thread found it needs,
gets one file in `docs/planning/asks/<slug>.md`:

```yaml
---
ask: Export three CR3 and three ProRAW files for photo-import's RAW tests
kind: do                     # decide | review | do
category: files              # optional; a word or a list
from: "Claude project thread: photo-import state assessment"
unblocks: photo-import RAW support    # optional; a string or a list
raised: 2026-10-17
defer_until: 2026-10-27      # optional; hidden from the open list until then
---

Context: what is needed, why, and links to the thread, PR or doc.
```

When it is answered, add `answered:` (a date), `outcome:` (`done`,
`decided`, `rejected`, `automate` or `dropped`) and `answer:` (one line,
with a link to whatever records the result), and `git mv` it to
`docs/planning/asks/archive/`. For `automate`, the answer names the
backlog item or dispatch that will build the command.

Ask files come from whoever finds the ask: a thread that needs something
it cannot put in its own dispatch doc, the conductor of a plan, or the
weekly planning session. If an ask belongs to a doc, it goes in that doc.
A human gate goes in its dispatch doc, and a plan gate goes in its plan.

## The status tool

```bash
scripts/asks status --fetch        # the queue: Decide, Review, Do, then deferred and candidates to automate
scripts/asks status --json         # the same, for an agent
scripts/asks status --all          # also list RFC drafts untouched for 60+ days
scripts/asks check                 # validate ask files; warn about gate entries with no matching task
```

Within each kind, asks are ordered by how many plan threads they unblock,
then by whether their dispatch's earlier tasks are all marked done, then by
age. The age is how long ago the dispatch doc or ask file was added, or how
long an RFC at `considering` has been idle. Ages need full history, so the
tool says when the clone is shallow.

Kinds and categories of dispatch gates are inferred from the gate's
wording, in order: devices, a console, a password manager, network grants
and restore drills make it `do`; "accept", "confirm" and "design" make it
`decide`; "review", "check" and "verify" make it `review`; "approve",
"production", "converge", "upgrade" and "bootstrap" make it `approve`;
other action verbs make it `do`. An ask
file states its own. Run `check` with the repo's fast tests.

The tool shares its git and dispatch parsing with `scripts/plan`, and like
it needs only git and PyYAML.

## The sweep

Open PRs and agent threads are not in git, so the full queue is put
together by an agent that can see all three:

1. `scripts/asks status --fetch --json`.
2. Open PRs on the repos (GitHub): the ones waiting on the operator's
   review or merge, each with its CI state and anything else it waits on.
   A draft RFC PR is a `review` ask, because the RFC needs reading before
   it can be decided.
3. Agent threads (a Claude project's threads, or wherever agents report)
   that are blocked or review-ready: what each needs from the operator, in
   one line, with a link. An unanswered decision card is a `decide` ask.
4. Merge the three lists. An ask seen in more than one place, such as a
   dispatch gate a thread is also asking about, is listed once with both
   links.
5. Report the result grouped by kind, most unblocking first, with
   candidates to automate and stale gates at the end.

The sweep (`/ordvik:asks sweep`) runs as a step of every planning session
(`/ordvik:planning`, whose report carries its sections) and any time the
operator asks what needs them. It changes nothing by itself. Stale gates,
answered asks and new automation items are closed or added through the
planning session's PR.

## Turning recurring asks into commands

`scripts/asks status` ends with **Candidates to automate**: every category
with three or more open `do` asks. `approve` asks are not counted, because
the operator's approval is the part that should stay. Each planning session looks at that list
and, for each candidate, decides one of:

- **Automate**: add a backlog item for a command that does the step, with
  the operator approving a dry run instead of typing commands. The asks
  that prompted it point at the item.
- **Keep**: the step genuinely needs a person, such as handling a device or
  making a judgement. Say why in the planning record so it is not raised
  again every week.

## Why not more

- **No queue service.** Asks live in the docs that own them and in a small
  directory of files. The sweep needs no state of its own.
- **No GitHub API in the tool.** It runs the same in a cloud container, on
  an agent VM and on a laptop. The sweep adds PRs, because the agent running
  it has GitHub access anyway.
- **No copying gates into ask files.** A gate that is stale gets fixed in
  its dispatch doc, which is where the next reader looks.
