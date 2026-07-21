---
id: 6
title: Registry — projects and secrets store
role: backend
depends: [1]
status: todo
---
Persist the one thing FlightDeck legitimately owns: which projects are registered and
their secrets. SQLite-backed, secrets never exposed (ADR-005). No ticket status here ever.

## Likely files
- `internal/registry/{registry,sqlite,migrations}.go`

## Acceptance criteria
- [ ] CRUD for `Project` (add/list/get/remove) backed by SQLite via `modernc.org/sqlite`
      (no cgo); migrations embedded, idempotent on open.
- [ ] Per-project secrets (routine token, github token) stored separately and returned
      only to server-side callers; a `Project` DTO for the API carries no secret.
- [ ] The DB file path is configurable and gitignored; a store round-trip test runs against
      a temp file, not `:memory:`.
- [ ] No table stores ticket status (ADR-001); a test asserts the schema has no such column.
- [ ] lint + tests pass.

## Handoff
Note the API-facing Project DTO shape (secret-free) the api ticket serves.
