---
id: 7
title: Dispatch — routine fire, autopilot, approve-merge
role: backend
depends: [1]
status: done
---
Implement `core.Dispatcher`: trigger a project's routine, read/flip its autopilot switch,
and approve a merge — all on explicit request, never automatically.

## Likely files
- `internal/dispatch/{fire,autopilot,merge}.go`

## Acceptance criteria
- [ ] `Fire` POSTs to the routine `/fire` endpoint with the per-project bearer token and
      the ticket id in the payload, returning the session URL from the response.
- [ ] `Autopilot`/`SetAutopilot` read and write `.claude/autopilot.json` in the target repo
      (or via its remote), preserving the file's other fields.
- [ ] `ApproveMerge` merges a PR via the GitHub API **only when called**; there is no code
      path that merges without an explicit ApproveMerge call. A test asserts dispatch and
      merge are separate and neither implies the other.
- [ ] HTTP injectable; tests use a stub transport, no live calls. Tokens never logged.
- [ ] lint + tests pass.

## Handoff

Package `internal/dispatch` implements `core.Dispatcher` — the write side. Ticket 008 (api)
constructs one per project (tokens from `registry.Secrets`) and wires it to the dispatch /
autopilot / approve endpoints. Everything here happens ONLY on an explicit method call; there
is no server-side automation (CLAUDE.md).

**Constructor:**
```go
func New(routineToken, githubToken string, opts ...Option) *Client
// Options: WithHTTPClient(*http.Client) (test injection), WithRoutineBaseURL(string)
```
`routineToken`/`githubToken` come from `registry.Secrets{RoutineToken, GitHubToken}` for the
project. Set the routine base URL per project via `WithRoutineBaseURL` (default is a
placeholder). Tokens ride only the `Authorization: Bearer` header — never logged, never in an
error (asserted). Implements `core.Dispatcher` (compile-time asserted).

**`Fire(ctx, p, ticketID) (sessionURL string, err error)`** — POSTs to `<routineBase>/fire`
with body **`{"ticket_id": <int>}`** and returns the response's **`{"session_url": "..."}`**.
These are exactly the field names in ARCHITECTURE.md's frozen contract, so the api handler for
`POST /api/projects/{id}/dispatch` passes its inbound `{ticket_id}` straight in and returns
`{session_url}` straight out — **`session_url` is the field the frontend surfaces** (it opens
the live session). On any failure returns a wrapped `ErrDispatchFailed`; **one attempt, never
retried** — surface the error to the user (ARCHITECTURE.md failure policy).

**`Autopilot(ctx, p) (on bool, err error)` / `SetAutopilot(ctx, p, on bool) error`** — read /
write `p.RepoPath + "/.claude/autopilot.json"`. `SetAutopilot` flips only `enabled` and
preserves every other field (`maxInFlight`, `note`, unknowns) byte-for-byte via
`map[string]json.RawMessage`. Backs `GET/PUT /api/projects/{id}/autopilot`. (Remote-repo path —
a project registered with no local clone — is out of scope here; local `RepoPath` only.)

**`ApproveMerge(ctx, p, prNumber int) error`** — squash-merges the PR via GitHub
(`PUT /repos/{owner}/{repo}/pulls/{n}/merge`) using the github token, **only when called.**
Backs `POST /api/projects/{id}/tickets/{tid}/approve` — a **human-action-only** endpoint.
`Fire` and `ApproveMerge` are entirely separate code paths (a recording-transport test proves
Fire issues no `/merge` and ApproveMerge issues no `/fire`); the api must keep them on separate
routes and never call `ApproveMerge` implicitly.

Imports only `core` + stdlib — no new dependency.
