---
id: 1
title: Foundation — core domain types and interfaces
role: backend
depends: []
status: todo
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
