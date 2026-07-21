---
name: plan
description: Turns docs/PRD.md into an architecture sketch and a set of independent tickets sized for one agent each.
argument-hint: [optional: which part of the PRD to plan]
disable-model-invocation: true
allowed-tools: Read, Write, Edit, Glob, Grep, Bash(git *)
---

Scope: $ARGUMENTS

Read `docs/PRD.md` first. If it does not exist, stop and say to run `/spec` — planning
without acceptance criteria produces tickets that nobody can verify.

**Write no implementation code in this command.** You are producing a plan; `dev` writes
the code. If you find yourself drafting a function body, you have gone too far.

## Step 1 — Architecture

Read enough of the existing code to know what is already there. Then sketch, in
`docs/ARCHITECTURE.md` (template at `docs/ARCHITECTURE-template.md` if present):

- The shape of the change: which modules, which boundaries, what talks to what
- Decisions that are hard to reverse later, each with the alternative you rejected and why.
  These belong in `docs/decisions/` as ADRs — one file per decision
- What you are deliberately *not* building yet

Where you are guessing about the existing system, say so rather than presenting a guess
as a reading.

## Step 2 — Tickets

Split the work so each ticket is:

- **Independent.** Two tickets touching the same file cannot run in parallel — that is the
  overwrite scenario the whole worktree setup exists to prevent. If two pieces are coupled,
  make them one ticket with an ordered plan inside, not two.
- **One agent's worth.** If a ticket needs three checkpoints and a day, split it.
- **Verifiable.** Carries its own acceptance criteria, traced back to the PRD.

## Step 3 — Show the tickets, then write them to docs/tickets/

First print the full set for review, in this shape:

```
### <short title>
Role:        designer | frontend | backend | qa | dev
Files:       <likely paths>
Depends on:  <ids, or none>
Acceptance criteria:
  - ...
```

Assign the role by what the ticket produces, not by which files it touches: design
system and component look → `designer`; screens and client logic → `frontend`; APIs,
storage, pipelines → `backend`; test suites proving criteria → `qa`; cross-cutting or
unclassifiable → `dev`. Sequence specialists through `depends`: designer tickets come
before the screens that consume them, contract-defining backend tickets before both, and
a `qa` ticket after the features it proves.

**Confirm with the human before creating anything.** A batch of wrong ticket files is
tedious to clean up, and ids, once referenced by `depends`, should not be renumbered.

Once confirmed, write one file per ticket to `docs/tickets/NNN-<slug>.md` in the format
`docs/tickets/README.md` defines — zero-padded id continuing from the highest existing
one, frontmatter with `id`, `title`, `role`, `depends`, `status: todo`, and the body
carrying the likely files and acceptance criteria verbatim. The body **is** the contract
`execute-ticket` and `validator` work from; a ticket without checkable criteria will be
refused and bounce straight back.

Number in dependency order: foundations get low ids, dependants higher — `next-ticket`
dispatches lowest-first, so the numbering is the execution sequence.

Commit the ticket files (and the architecture docs) to main in one commit. No labels, no
issue tracker, no API — the files are the queue, on any hosting or none.

## Step 4 — Order and parallelism

State which tickets can start at once and which wait — `/board` will show the same thing
from the files. Recommend how many to dispatch in parallel: three to five is the working
range, not "all of them". Parallelism is bounded by review throughput and by plan usage
limits, not by how many tickets exist.

End with what you are least sure about in this plan. That is the part most worth a human
looking at before any agent starts.
