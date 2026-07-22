---
id: 8
title: REST API and SSE
role: backend
depends: [5, 6, 7]
status: done
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

Package `internal/api` serves the frozen contract as-frozen — every endpoint's JSON matches
`ui/src/lib/types.ts` byte-for-byte (verified field-by-field), so **the frontend needs no
changes**. Ticket 013 (wiring/serve/embed) is the composition root that builds and mounts this.

**Mount it:**
```go
srv := api.NewServer(api.Config{
    Token:      os.Getenv("FLIGHTDECK_TOKEN"),        // required; constant-time compared
    Registry:   store,                                 // *registry.Store satisfies api.ProjectRegistry
    Source:     api.NewGitHubSource(store),            // real ProjectSource: git+github+derive, store as SecretsReader
    Dispatcher: api.NewDispatcherFactory(store),       // per-project dispatcher, tokens from the registry
    // Events:   optional *api.Broker; nil => a fresh one. Pass your own to Publish from a background loop.
})
http.Handle("/api/", srv.Handler())   // srv.Handler() is the chi router; mount the embedded UI (ticket 013) alongside
```
`*registry.Store` (ticket 006) satisfies `api.ProjectRegistry` (Add/List/Get/Remove) and
`api.SecretsReader` (Secrets) directly. `api.NewGitHubSource`/`NewDispatcherFactory` take the
store as the secrets source. `srv.Events()` returns the `*Broker` for publishing
`board.changed`/`ci.changed` from a background refresh loop (none is built yet — see additions).

**Auth flow for 013/frontend:** the UI POSTs `Authorization: Bearer $FLIGHTDECK_TOKEN` to
`/api/session`, gets an httpOnly cookie, and every later request carries the cookie. A valid
bearer also works directly. `FLIGHTDECK_TOKEN` must be set in the server env (ticket 013 reads
it); if empty, auth cannot succeed — 013 should require it at startup.

**Contract additions / deviations (the ticket asks to note these):**
1. **`core.Project`, `core.Ticket`, `core.PRState`, `core.BoardTicket` gained `json:` struct
   tags** (additive, no field/logic change) so encoding/json emits the snake_case names and
   flattens the embedded Ticket exactly as `types.ts` expects. Guard tests still pass; still no
   Status field.
2. **`internal/source/git.ParseStatus(fileContent) (string, error)`** was exported (ticket 005's
   handoff predicted this) to read a ticket file's `status:` on an arbitrary branch.
3. **`GET /api/agents`** synthesizes `AgentSession[]` from `in_progress` tickets (branch exists,
   file still todo) across projects. `started_at`/`last_activity_at` come from the branch tip's
   commit time (`ProjectSource.BranchCommitTime`); **`session_url` is empty** — routine session
   URLs from `Fire` are not persisted in v1 (no store for them, by design). If a project needs
   real session URLs later, that needs a small session store; out of scope here.
4. **SSE**: `dispatch.started` is published when `Fire` succeeds. `board.changed`/`ci.changed`
   have full broker + SSE plumbing and are unit-tested, but **nothing publishes them
   automatically yet** — a background poll loop that diffs derived boards (or a git/webhook
   hook) is the intended publisher; wire it in 013 or a follow-up, passing your own `Broker` via
   `Config.Events`. The frontend already subscribes to all three.
5. New dependency: `github.com/go-chi/chi/v5 v5.3.1` (`go.sum` committed).

Board is `derive.Board(derive.Derive(...))` computed per request — never cached, never stored.
GitHub failure downgrades PRs to `unknown` and still serves the git-derived board.
