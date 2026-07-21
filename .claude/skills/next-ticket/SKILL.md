---
name: next-ticket
description: Picks the next dispatchable ticket from docs/tickets/ and implements it, without anyone naming which one. Claims by pushing the ticket branch, so concurrent runs cannot take the same ticket.
argument-hint: "[max tickets in flight, default 1]"
allowed-tools: Read, Glob, Write, Edit, Grep, Bash, Agent
---

Requested: $ARGUMENTS

The queue lives in `docs/tickets/` — statuses are derived from git, never stored
elsewhere. See `docs/tickets/README.md` for the model.

## 0. Read the kill switch

Read `.claude/autopilot.json`.

- `maxInFlight` from that file is the limit for step 1, unless the request passed a
  number, which wins.
- If this run was started by a routine or asked for **autopilot**, and `enabled` is
  `false`, **stop immediately**. Say autopilot is off and that `/autopilot on` turns it
  back on. Do not work the ticket anyway.

A human asking for one ticket directly in a session is not autopilot — serve that
whatever the flag says. The switch governs unattended execution, not the skill.

## 1. Check whether there is room

In flight = tickets whose branch `claude/NNN-*` exists while the file in main still says
`todo`. Count them (remote branches too, when a remote exists). At or above the limit —
**stop** and report what is in flight. This check is what keeps two runs from claiming
the same ticket.

## 2. Pick the next ticket

Ready = `status: todo` in main, every id in `depends` is `done` in main, and no
`claude/NNN-*` branch exists. Take the **lowest id** — the planner numbers tickets in
dependency order, so lowest-first is the intended sequence. Skip any ticket whose branch
carries `needs-attention`: a human must look at those before a retry.

## 3. Claim it by pushing the branch

```
git checkout main && git pull
git checkout -b claude/NNN-<slug>
git push -u origin claude/NNN-<slug>    # skip the push when there is no remote
```

The push **is** the lock: if it fails because the branch already exists, another run got
there first — say so and pick the next ready ticket (once). With no remote, creating the
local branch is the claim.

## 4. If nothing is ready

Stop and say which of these it is — the distinction tells the human their next move:

- **Blocked work exists** — name the blocked tickets and the unmerged tickets holding them.
- **Everything is in review** — the queue advances on merges; that is the human's move.
- **Something needs attention** — name it; it will never clear on its own.
- **Nothing left** — every ticket is done. Say so plainly.

Do not invent work, and do not take a blocked ticket because it looks ready to you.

## 5. Run it

Invoke the `execute-ticket` skill for the chosen id. Everything about how the ticket is
implemented lives there — do not restate or second-guess it here.

Handle exactly one ticket per run. The chain advances when a merge fires the next run,
so review sits between tickets by construction.
