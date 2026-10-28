---
name: ask
description: Ask the operator for a decision in one call, with two to four options and a recommendation, written to the planning repository; then read the answer back. Use when a thread needs a choice only the operator can make, instead of asking in thread prose.
allowed_tools:
  - Read
  - Bash(ordvik planning ask *)
  - Bash(ordvik planning asks *)
  - Bash(scripts/ordvik planning ask *)
  - Bash(date *)
---

# Ask the Operator

A thread that needs the operator to choose writes one **ask** to the
planning repository with `ordvik planning ask new`, says one line in its
thread pointing at it, and keeps working. The operator answers it wherever
they read asks (the `asks` list, an inbox that reads the planning
repository, or `ordvik planning ask answer`), and the answer is a commit
in the same repository, so the thread reads it back with `ask show` or
`ask wait`. Git is the store: nothing but a push to the planning
repository is needed, from a laptop, an agent VM or a cloud session.

The ask's format and where asks live are in the asks skill
(`/ordvik:asks`, `references/asks.md`).

The CLI is `ordvik planning` (`go install github.com/xythian/ordvik/cmd/ordvik@latest`,
or `scripts/ordvik` in a repository that has it), run in a checkout of the
planning repository (`$ORDVIK_PLANNING`, or `planning/` beside the code
checkout). It writes in a temporary worktree on the tip of `main`, so the
checkout may be on any branch; a refused push is retried on the new tip.

## Before asking

- **Don't ask for what you can decide or undo yourself.** A step that can
  be undone after the operator sees it (a branch, a PR, a query, a file
  edit) is yours: do it and say what you did. Ask only for a choice that
  changes the goal, or a step that cannot be undone.
- **One ask per decision.** Two questions are two asks, so each gets its
  own answer.
- **Put it where it belongs.** A decision a dispatch's task waits on goes
  on that dispatch's gate (`--gate <project>/<dispatch>:<task>`), and a
  plan gate's decision on the plan's gate (`--gate plan:<project>/<plan>:<gate>`).
  Anything else gets its own ask file (`--project`).

## Writing a good ask

- **Lead with the question**, verb first, in the operator's words, short
  enough to read on a phone: "Ship the new parser behind a flag?"
- **Two to four options**, each a short label and what happens if it is
  chosen (`--option "label=then"`). The `then` is the consequence, not a
  restatement of the label.
- **Recommend one** (`--recommended`, the option's index from 0) and say
  why in the context.
- **Context in a few sentences** (`--context`, at most 120 words): why it
  is asked now and what each answer changes. Link the PR or doc rather than
  pasting it.
- **Say where it came from**: `--from-thread` (the thread's title or link),
  `--from-pr owner/name#n` (an answer can then be delivered to the PR, which
  wakes a thread subscribed to it), `--from-dispatch <project>/<id>`.
- `--kind` is `decide` unless the ask is a `review`, an `approve` (a dry
  run an agent then runs) or a `do`; `--category` names the kind of step,
  so recurring asks can be counted.

```bash
ordvik planning ask new --project demo \
  --question "Ship the new parser behind a flag?" \
  --option "Behind a flag=the old parser stays the default for a week, then goes" \
  --option "Replace now=the old parser is deleted in this PR" \
  --recommended 0 \
  --context "Two parser bugs were fixed this week. A flag lets the operator switch back without a release." \
  --from-thread "Parser rewrite" --from-pr acme/app#12 --category parser
```

It prints the ask's id (`ask:demo/ship-the-new-parser-behind-a-flag`) and,
when the planning repository's `planning.yml` sets `ask_url`, a link to it.

## After asking

1. Say **one line** in the thread pointing at the ask: the question, your
   recommendation and the id or link. Don't repeat the options in prose.
2. **Keep working on the recommended option**, unless the step cannot be
   undone. Then work on whatever does not depend on the answer.
3. Read the answer when you need it:
   - `ordvik planning ask show <id>` prints the ask and its answer (`--json`
     for `answered`, `answer` (the option's index, or `other`),
     `answer_text` and `answered_by`).
   - `ordvik planning ask wait <id> --timeout 10m` fetches until it is
     answered (exit 0) or the timeout passes (exit 3, still open). Don't
     hold a session in a long wait: end the turn and check again when
     something wakes it.
4. Act on the answer. An `other` answer's `answer_text` says what the
   operator wants instead. If the answer was not the option you worked on,
   undo or redirect that work and say so in one line.

## Recording an answer

When the operator answers in words (in a thread, in chat), record it so the
ask leaves the list and the answer is in git:

```bash
ordvik planning ask answer <id> <n|other> [--text "their words"] [--by <name>]
```

An ask file gains `answered`, `answer`, `answer_text`, `answered_by` and
`outcome: decided` and moves to `asks/archive/`; a gate gains the same
fields in place. A dispatch gate's task is still marked `— DONE` when its
work is done, and a plan gate is still `passed:` by the plan's conductor.
