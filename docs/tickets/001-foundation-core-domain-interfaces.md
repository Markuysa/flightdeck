---
id: 1
title: Foundation — core domain types and interfaces
role: backend
depends: []
status: done
---
Bootstrap the Go module and `internal/core`: the domain types and every cross-package
interface, copied from `docs/ARCHITECTURE.md`. Nothing else compiles until this lands.

## Likely files
- `go.mod`
- `internal/core/{project,ticket,board,interfaces,errors}.go`

## Acceptance criteria
- [ ] `go build ./...` and `go test ./...` pass on a clean clone (Go 1.22+).
- [ ] `internal/core` defines `Project`, `Ticket`, `BoardTicket`, `PRState`, `DerivedStatus`
      exactly as in ARCHITECTURE.md, and the interfaces `TicketReader`, `GitState`,
      `PRReader`, `Dispatcher`.
- [ ] `Ticket` has **no** `Status` field — status is derived, never parsed as truth
      (ADR-001). A test asserts the struct has no such field.
- [ ] `internal/core` imports no other internal package, asserted by a test (ADR-004).
- [ ] `golangci-lint run` passes.

## Handoff

The Go module and the domain root now exist. Everything downstream imports from here;
nothing else needs to redefine these types.

**Module:** `github.com/Markuysa/flightdeck`, `go 1.22` (CI installs Go via
`go-version-file: go.mod`, so bump the directive, not a separate pin).

**Package `internal/core`** (imports stdlib only — never import another internal package
from here, a test enforces it):

- `internal/core/project.go` — `Project{ID, Name, RepoPath, Remote, Owner, Repo string}`.
  `Remote` is `"github"` or `""` (local-only). Secrets do **not** live here — the registry
  (ticket 006) holds routine/GitHub tokens.
- `internal/core/ticket.go` — `Ticket{ID int; Title, Role string; Depends []int; Body,
  Handoff string}`. **No `Status` field, by design (ADR-001)** — do not add one; parse the
  file's `status:` value only where the derive engine needs the two literals `done` /
  `needs-attention`, never onto this struct.
- `internal/core/board.go` — `DerivedStatus` (string) with the six constants the derive
  engine (ticket 005) must emit: `StatusReady` `"ready"`, `StatusInProgress` `"in_progress"`,
  `StatusInReview` `"in_review"`, `StatusBlocked` `"blocked"`, `StatusNeedsAttention`
  `"needs_attention"`, `StatusDone` `"done"`. `BoardTicket{Ticket; Status DerivedStatus;
  Branch string; PR *PRState}` and `PRState{Number int; URL string; CI string}` (CI is
  `pending|green|red|unknown`).
- `internal/core/interfaces.go` — the four cross-package contracts to implement, not
  redefine:
  - `TicketReader.Tickets(ctx) ([]Ticket, error)` → the disk ticket source (ticket 003 area).
  - `GitState.Branches(ctx) ([]string, error)`, `.IsMergedToMain(ctx, branch) (bool, error)`,
    `.FileOnBranch(ctx, branch, path) (string, error)` → git source (ticket 003).
  - `PRReader.OpenPRs(ctx) (map[string]PRState, error)`, **keyed by branch name** → github
    source (ticket 004).
  - `Dispatcher.Fire(ctx, p Project, ticketID int) (sessionURL string, error)`,
    `.Autopilot(ctx, p) (on bool, error)`, `.SetAutopilot(ctx, p, on bool) error`,
    `.ApproveMerge(ctx, p, prNumber int) error` → dispatch (ticket 007).
- `internal/core/errors.go` — sentinel errors `ErrProjectNotFound`, `ErrTicketNotFound`
  for registry/api to branch on with `errors.Is`. Add further domain sentinels here as
  needed rather than scattering `errors.New` in feature packages.

**Derive engine (ticket 005)** is `derive(tickets, mainStatuses, branches, openPRs) ->
[]BoardTicket` — a pure function over these types, so it unit-tests against fixtures with
no network. The status rules live in ARCHITECTURE.md §"Status derivation"; core only
supplies the vocabulary.

**Test conventions established:** ADR invariants are guarded by tests in the package they
constrain (`internal/core/core_test.go`). If you add a type to core, keep it stdlib-only or
`TestNoInternalImports` fails — that's intended.
