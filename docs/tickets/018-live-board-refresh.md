---
id: 18
title: Live board — background refresh publisher for SSE
role: backend
depends: [8, 13]
status: done
---
Close the gap where the SSE plumbing exists but nothing publishes `board.changed`/`ci.changed`.
Add a background loop in the composition root that re-derives each project's board on an interval
and publishes to the broker the server already streams from, so the Board/Agents screens update
live when a branch merges or CI flips — no auto-anything on the domain, just observe-and-notify.

## Acceptance criteria
- [ ] A refresher in `internal/app` holds the same `*api.Broker` the server streams from,
      polls every registered project on a configurable interval, and publishes
      `board.changed` when a project's derived board changed since the last poll, and
      `ci.changed` when only the PR/CI states changed.
- [ ] It publishes ONLY on an actual change (no per-tick storms), survives per-project errors
      (skip, never crash the loop), and shuts down cleanly with the app context.
- [ ] Interval is env-configurable (e.g. `FLIGHTDECK_REFRESH_INTERVAL`, sensible default);
      a value that disables it is honored.
- [ ] Deterministic test: a fixture project, a subscribed broker, one poll → baseline; mutate
      the repo (flip a ticket / add a branch) → next poll publishes `board.changed`. Offline.
- [ ] lint + tests pass. This observes and notifies only — it never dispatches or merges.

## Handoff

`internal/app/refresh.go` turns the SSE plumbing into live updates. `internal/app` now creates
one shared `broker := api.NewBroker()` and one shared `source := api.NewGitHubSource(store)`,
passes the broker to both `api.NewServer(Config{Events: broker})` and a `Refresher`, and starts
`refresher.Run(ctx)` in `App.Run` (only when the interval is enabled), joined via a WaitGroup so
no goroutine outlives `Run`.

**What it does:** every tick it `registry.List`s projects and, per project, calls
`source.BoardTickets` (the same read the API uses — git + optional GitHub + `derive.Derive`),
fingerprints the result into a `structural` hash (id/status/branch/PR number+URL) and a `ci`
hash (id + PR CI state), and compares to the last poll. **Precedence:** structural change →
`board.changed`; else CI-only change → `ci.changed`; exactly one event per project per tick,
first observation is a silent baseline, steady state is silent. Per-project read errors are
logged (no secrets) and skipped — the loop never crashes. It only reads and publishes — never
dispatches, merges, or writes (CLAUDE.md's "no auto-anything on the server").

**Config:** `FLIGHTDECK_REFRESH_INTERVAL` (`time.ParseDuration`), default `5s`; `"off"` or any
duration `<= 0` disables the refresher entirely; an invalid string is a fail-fast startup error.

The Board and Agents screens already `useFlightDeckEvents(() => refetch())`, so with this
publisher live they refresh within one interval of a branch/PR/CI change. Testable via the
exported `Refresher.pollOnce` (baseline → mutate fixture → `board.changed`), no wall-clock waits.
