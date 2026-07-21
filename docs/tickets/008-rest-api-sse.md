---
id: 8
title: REST API and SSE
role: backend
depends: [5, 6, 7]
status: todo
---
Serve the frozen contract over chi, composing derive + registry + dispatch, with SSE for
board/dispatch/ci changes. Single-token auth traded for a session cookie.

## Likely files
- `internal/api/{server,projects,board,ticket,dispatch,agents,events,auth}.go`

## Acceptance criteria
- [ ] Every endpoint in the ARCHITECTURE.md contract table implemented with its documented
      shape; `POST /api/session` sets the cookie from `FLIGHTDECK_TOKEN`.
- [ ] `GET /api/projects/{id}/board` returns the derived board; status is computed per
      request, never read from a store.
- [ ] `POST .../dispatch` is rejected (409) for a ticket that is not `ready`; `approve`
      merges only the named PR.
- [ ] Secrets never appear in any response body or log line; a test greps responses for
      token-shaped strings.
- [ ] SSE `/api/events` emits `board.changed`, `dispatch.started`, `ci.changed`.
- [ ] Handler tests use fake core interfaces; no network. lint + tests pass.

## Handoff
Confirm the contract is met as-frozen so the frontend needs no changes; note any addition.
