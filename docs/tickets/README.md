# The ticket queue

Tickets live here as files, one per ticket, named `NNN-slug.md`. The queue needs nothing
but git: no GitHub issues, no external tracker, no API. It works identically on GitHub,
GitLab, or a repository that never leaves your machine, and it travels into cloud sessions
with the clone.

## Format

```markdown
---
id: 3
title: Hacker News collector
role: dev            # which agent implements it (dev today; designer/qa as roles grow)
depends: [1]         # ids that must be done first; [] if none
status: todo         # todo | done | needs-attention — nothing else is ever stored
---

Context, likely files, and the acceptance criteria — same content an issue body carried.
Acceptance criteria are the contract: execute-ticket refuses a ticket without them.
```

## The status model: store two bits, derive the rest

Stored state is the thing that goes stale. So almost nothing is stored:

| Status | How it is determined |
|---|---|
| `done` | the file in **main** says `status: done` — which can only happen by merging the ticket's branch |
| `blocked` | any id in `depends` is not done in main |
| `ready` | `todo` in main, all depends done, no ticket branch exists |
| `in progress` | branch `claude/NNN-*` exists and the file **in that branch** still says `todo` |
| `in review` | branch exists and the file in that branch says `done`, while main still says `todo` |
| `needs attention` | the file in the ticket's branch says `needs-attention` (validator came back red; a human must look) |

The agent's last commit before opening the merge request flips the ticket file to
`status: done` **in its own branch**. Merging the branch is therefore what lands `done`
in main — the board cannot disagree with reality, because the board *is* reality.

**Claiming is pushing the branch.** A second run trying to claim the same ticket fails to
push the same branch name and moves on. With no remote (purely local work), creating the
local branch is the claim.

## Reading the queue

`/board` renders the whole queue with derived statuses. It needs only git:

```
git show origin/main:docs/tickets/003-hn-collector.md   # status in main
git branch -a --list '*claude/003-*'                    # is it claimed?
git show claude/003-hn-collector:docs/tickets/003-hn-collector.md  # its state in-branch
```

## Finishing a ticket per platform

The queue is platform-neutral; only the merge step differs:

- **GitHub**: `gh pr create`, and under autopilot `gh pr merge --auto --squash` once CI is
  green. Required status checks stay the gate.
- **GitLab**: `glab mr create`; merge trains / pipeline gates play the same role.
- **No remote / local**: the agent stops after committing `status: done` in its branch and
  tells you the branch name. You review and `git merge` it. Autopilot's auto-merge is
  impossible here by construction — there is no server to enforce a gate, so the human is
  the gate.
