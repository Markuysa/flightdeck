---
id: 19
title: Agent session URLs — remember dispatched sessions
role: backend
depends: [7, 8, 13]
status: done
---
Close the gap where GET /api/agents always returns an empty session_url. When a dispatch fires,
the routine returns a session URL; remember it so the Agents view can link to the live session
of an agent this server dispatched. Ephemeral by nature (live sessions), in-memory only.

## Acceptance criteria
- [ ] A concurrency-safe in-memory session store on the api server records the session URL and
      dispatch time when Fire succeeds, keyed by (project id, ticket id).
- [ ] GET /api/agents fills an in_progress ticket's `session_url` (and `started_at` from the
      recorded dispatch time when known) from that store; agents whose branch was not dispatched
      through this server simply have no URL — honest, not fabricated.
- [ ] No secret is stored or leaked; the session URL is not a secret but tokens still never
      appear. Thread-safe under `-race`.
- [ ] Tests: after a dispatch, the corresponding agent row carries the session_url; an agent
      with no recorded dispatch has an empty URL. lint + tests pass.
- [ ] Frontend already renders the Session link only when session_url is non-empty (ticket 012)
      — no frontend change required; confirm it lights up end to end.

## Handoff

`GET /api/agents` now carries a real `session_url` for agents this server dispatched.

- `internal/api/sessions.go` — `dispatchSessionStore`: an in-memory, `sync.RWMutex`-guarded map
  keyed by `(projectID, ticketID)` holding `{sessionURL, dispatchedAt}`. `Record` / `Lookup`.
  Named `dispatchSessions` to avoid colliding with the auth session-cookie store in `auth.go`.
  Initialized in `NewServer`. **In-memory only** — a session URL is neither ticket status nor a
  secret, but it isn't durable-store material either (ADR-001/CLAUDE.md); a restart forgets it,
  which is correct for live sessions.
- `handleDispatch` records `(project, ticket) → {url, now}` only when `Fire` succeeds.
- `handleListAgents` looks the store up per synthesized in_progress agent: when found, sets
  `session_url` and overrides `started_at` with the recorded dispatch time (more accurate than
  the branch tip commit); `last_activity_at` stays the branch commit time. Not found → empty
  `session_url` (honest — we only know URLs for sessions dispatched through this server).
- Thread-safe under `-race` (the refresher from ticket 018 and request handlers run
  concurrently). No token ever passes through the store; the redaction test still covers the
  dispatch→agents path.

**End to end:** the frontend `AgentRow` (ticket 012) already renders the Session link only when
`session_url` is non-empty, so it lights up automatically once an agent has been dispatched
here — no frontend change. Closes the last of the four v1 gaps.
