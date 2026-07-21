---
name: unblock
description: Reports which tickets became dispatchable after the latest merge. With file-based tickets statuses are derived, so there is nothing to relabel — this is a read-only report kept for routine prompts that call unblock before next-ticket.
allowed-tools: Read, Glob, Bash(git *)
---

Statuses in `docs/tickets/` are **derived from git**, so a merge unblocks dependants by
itself — there are no labels to move and nothing here may edit anything.

What remains useful is the report:

1. `git pull` main, read every ticket's frontmatter.
2. Find tickets that are now **ready** (todo, all `depends` done in main, no branch) and
   say which merge freed them — name the dependency that just became done.
3. Name what is still blocked and on which unmerged tickets.
4. If anything sits at `needs-attention`, say so: those never clear without a human.

One short block of output, no edits, no branches, no merges. If nothing changed, one
line saying so — that is a normal outcome.
