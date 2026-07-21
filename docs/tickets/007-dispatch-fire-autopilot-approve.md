---
id: 7
title: Dispatch — routine fire, autopilot, approve-merge
role: backend
depends: [1]
status: todo
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
Note the fire payload shape and the session-url field the frontend surfaces.
