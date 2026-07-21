---
id: 5
title: Derive engine — the status core
role: backend
depends: [1]
status: todo
---
The heart of the product: a pure function turning (tickets + main statuses + branches +
open PRs) into `[]BoardTicket`, following the derivation rules in ARCHITECTURE.md /
`docs/tickets/README.md`.

## Likely files
- `internal/derive/{derive,board}.go`

## Acceptance criteria
- [ ] `Derive(tickets, mainStatus, branches, prs)` implements every rule: done, needs_attention,
      in_review, in_progress, blocked, ready — with the exact precedence in ARCHITECTURE.md.
- [ ] It reads a ticket's stored `status` only for the literals `done`/`needs-attention`
      and computes everything else; a stored `in_progress`/`ready`/`blocked` is ignored.
      A test proves a lying `status: ready` on a ticket with an unmet dependency still
      derives `blocked`.
- [ ] Pure and total: no I/O, no clock, deterministic ordering by id. Tested with fakes
      only (fixture helper from #3), full branch coverage of the rule table.
- [ ] lint + tests pass.

## Handoff
This is what the API serves; note the BoardTicket ordering/grouping the frontend expects.
