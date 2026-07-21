# ADR-003: A Source interface so GitLab can join later

Date: 2026-07-21
Status: accepted

## Context
v1 ships GitHub only (PRD §5), but the queue model is host-neutral by design and GitLab
support is a stated v2 goal. Wiring GitHub calls directly into the derive engine would
make that a rewrite.

## Decision
Git state, PR/CI state, and dispatch are behind interfaces in `internal/core`
(`GitState`, `PRReader`, `Dispatcher`). The GitHub implementations live in
`internal/source/github`; the derive engine depends only on the interfaces. A GitLab
implementation is a new package, not a change to derive.

## Consequences
- derive is tested against fakes, no network.
- v1 has one implementation; the seam costs a little indirection now to avoid a rewrite later.
