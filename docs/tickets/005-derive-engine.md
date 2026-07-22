---
id: 5
title: Derive engine — the status core
role: backend
depends: [1]
status: done
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

Package `internal/derive` is the status engine. Ticket 008 (api) calls it to serve
`GET /api/projects/{id}/board` and the ticket-detail endpoint; it is a pure function, so
008 owns the I/O of assembling its inputs.

**`Derive(tickets, mainStatus, branches, prs) []core.BoardTicket`** — pure, total, no I/O,
returns one BoardTicket per ticket sorted by id. Inputs the api layer must assemble from the
sources:
- `tickets []core.Ticket` — from `git.TicketsWithStatus` (drop the RawStatus into `mainStatus`).
- `mainStatus map[int]string` — ticket id → the raw `status:` **on main** (`git.TicketsWithStatus`
  reads the working checkout = main). Missing entry is treated as `"todo"`.
- `branches map[int]BranchState` — present ONLY for a ticket that has a `claude/NNN-*` branch.
  `BranchState{Name string; FileStatus string}` where `FileStatus` is the raw `status:` of the
  ticket file **on that branch**. Assemble by: `git.Branches` to find which `claude/NNN-*`
  branches exist → map each to its ticket id → `git.FileOnBranch(ctx, branch,
  "docs/tickets/NNN-*.md")` → parse the frontmatter `status:`.
  **`git`'s frontmatter parser is currently unexported** (`parseFrontmatter`); ticket 008 needs
  a raw status for arbitrary file content, so 008 should export a small helper from
  `internal/source/git` (e.g. `ParseStatus(content string) (string, error)`) or add a
  `StatusOnBranch` method — that is 008's job, noted here so it is not a surprise.
- `prs map[string]core.PRState` — from `github.OpenPRs` (keyed by branch). **May be nil** when
  GitHub is unavailable; Derive treats nil as "no annotations", never an error. Wrap the
  github call so an `ErrGitHubUnavailable` downgrades to `nil`/empty, per ARCHITECTURE.md.

**Precedence (exact):** done (main==done) → needs_attention (branch FileStatus needs-attention)
→ in_review (branch FileStatus done) → in_progress (branch, else) → blocked (no branch, a
depend not done on main) → ready (no branch, all depends done). A dependency counts as done
only when merged to main (its own branch never satisfies a `depends`). `BoardTicket.Branch` is
set whenever a branch exists; `BoardTicket.PR` is set only for in_review with a PR present.

**Board grouping — what the board endpoint serializes.**
`Board(tickets []core.BoardTicket) map[core.DerivedStatus][]core.BoardTicket` groups Derive's
output by status. Every one of the six statuses is present as a key with an empty (never nil)
slice → JSON `[]`, not `null` — matching the frontend's `Board = Record<DerivedStatus,
BoardTicket[]>` exactly, so the UI indexes any status without a nil check. Ticket order within a
bucket is id order (Derive already sorts; Board preserves it). Group/column display order is
**`derive.StatusOrder`** = `[ready, in_progress, in_review, needs_attention, blocked, done]`,
identical to the frontend's `STATUS_ORDER`; the map carries no order, so serve `StatusOrder`
alongside (or rely on the client's constant). Pass `Board(...)` straight through as the board
JSON body.
