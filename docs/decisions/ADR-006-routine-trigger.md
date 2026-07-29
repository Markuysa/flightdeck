# ADR-006: Dispatch runs a named Claude routine trigger

Date: 2026-07-27
Status: accepted

## Context
Dispatch is the product's one write action that starts work, and it did not function in any
deployment. `internal/dispatch` posted to `<base>/fire` where `base` defaulted to
`https://routines.claude.local` — a non-routable placeholder. The option that overrides it,
`WithRoutineBaseURL`, was called only from tests: the composition root never passed one, and
there was nowhere to put a real value anyway. No column in `projects`, no field on
`core.Project`, no field in `CreateProjectRequest`, no form input, no environment variable.
Every dispatch failed with a DNS error.

Two things had to be decided, not one.

**Where the endpoint comes from.** The execution model is Claude routines: a routine is
created in Claude, and FlightDeck runs it so it picks the work up autonomously. The
remote-trigger API addresses a routine by **trigger id** — `POST
/v1/code/triggers/{trigger_id}/run` — not by a per-project base URL. So "make the base URL
configurable" would have been the wrong fix: the varying part is an id, and the host is
constant.

**How much to trust the response shape.** The host, path prefix, and the field naming the
live session link were read off the remote-trigger API's documented shape. They were not
verified from this codebase against a live trigger. Guessing wrong in a `const` would mean a
rebuild to correct.

## Decision
- `core.Project` gains `RoutineTriggerID`, persisted in the registry (additive
  `ALTER TABLE` migration — `CREATE TABLE IF NOT EXISTS` is a no-op against a database an
  older FlightDeck created, so the column list in `migrations.go` must be kept in step).
  It is an identifier, not a secret: the routine's bearer token stays in `project_secrets`
  (ADR-005), so the trigger id is safe on the DTO the browser already receives.
- `Fire` posts to `<base>/v1/code/triggers/{trigger_id}/run`, path-escaping the id — it
  arrives from an operator-filled form field and must not climb out of its path segment.
- The base is `dispatch.DefaultRoutineAPIBase`, overridable per deployment with
  `FLIGHTDECK_ROUTINE_API_BASE`. It is a `var`, not a `const`, precisely because it is
  unverified: a wrong host is an environment variable away from fixed, not a release away.
- The session link is parsed leniently — `session_url`, `url`, `routine_url`, `web_url`,
  `session.url`, first non-empty wins. A response with none of them is a **successful**
  dispatch with no link, never an error: the routine started, and a missing convenience must
  not be reported as a failure.
- A project with no trigger id fails with its own sentinel, `ErrNoRoutineTrigger`, which the
  API answers **409**, not 502. "You have not wired a routine up yet" and "the routine
  rejected us" send an operator to entirely different places.

## Consequences
- Dispatch works. Verified end-to-end against a stub trigger server: FlightDeck issued
  `POST /v1/code/triggers/trg_live_test/run` with `Authorization: Bearer <routine token>` and
  body `{"ticket_id":5}`, and returned the stub's `session_url` to the caller.
- **The one thing still unconfirmed** is the live contract: base host, and which field really
  carries the session link. Both are absorbed by the design above (env var; lenient parsing),
  but the first real dispatch should be watched, and `DefaultRoutineAPIBase` corrected in code
  once it is known.
- Registering a project is the only way to set the trigger id; there is no update endpoint.
  Correcting it means re-registering, which costs nothing — the registry stores no ticket
  state (ADR-001) — but it is a rough edge worth closing when project editing arrives.
- Routines are not load-bearing on the architecture. `core.Dispatcher` is the seam (ADR-003's
  reasoning applied to the write side): replacing routines with another execution backend
  means a new implementation of that one interface. Nothing outside `internal/dispatch` names
  a trigger endpoint.
