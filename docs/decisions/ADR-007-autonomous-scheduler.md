# ADR-007: The server dispatches on its own, and records what it ran

Date: 2026-07-27
Status: accepted

Supersedes the "no auto-anything on the server" constraint stated in `internal/dispatch`
and `internal/app/refresh.go`. It does **not** supersede ADR-001.

## Context
FlightDeck's stated purpose is a queue that drains without a human clicking each ticket:
describe the work, start it, and let a team of agents execute. What was built dispatched
one ticket per human click. Every step forward — start ticket 5, then 6, then 7 as their
dependencies merged — needed a person watching the board.

The rule that made it so ("the dashboard dispatches and merges only on explicit human
action; no auto-anything on the server") was a good default while the write path was
unproven. It is also the single thing standing between the current product and its goal.
Keeping it means the product cannot do what it exists to do.

Two problems had to be solved together, because the second one makes the first unsafe.

**Deciding what to start.** Already solved, as it turns out. `derive` computes `ready` as
"no branch, the file says todo, and every dependency is done on main" — which is exactly
"dispatchable right now". The scheduler does not re-derive anything; it reads the same
board the UI reads.

**Not starting it twice.** Firing a routine does not change the board. The ticket stays
`ready` until the agent pushes a `claude/NNN-*` branch, which can take minutes. A
scheduler that looked only at the board would re-fire the same ticket on every tick for
the whole of that window — a fork bomb pointed at your own agent budget. Nothing in git
records "we already asked for this", so nothing derived can close the gap.

## Decision

**A scheduler loop (`internal/schedule`), off by default.** `FLIGHTDECK_SCHEDULE_INTERVAL`
is the only `FLIGHTDECK_*` variable where unset does not mean "use a sensible default":
unset, empty, `off`, or `<= 0` all mean disabled. Observing is free; dispatching is not.
Per project, the switch is the **existing autopilot toggle** — it already means
"unattended progress is allowed here", so it stays one switch rather than two that mean
almost the same thing. An unreadable `autopilot.json` counts as off: unreadable is never
"go ahead".

**A `runs` table (`internal/registry/runs.go`).** One row per dispatch attempt: project,
ticket, attempt number, state (`running` / `observed` / `timed_out` / `failed`), session
URL, timestamps. A ticket with an active run is never a candidate, whatever the board
says. Both dispatch paths write it — a human dispatch that left no trace would be
invisible to the scheduler, which would then fire the same ticket itself.

**This does not violate ADR-001.** A run says *"this server ran routine R for ticket 7 at
12:04, and here is how that attempt ended"*. That is a fact about FlightDeck's own
actions, and it is unavailable from git by construction — no branch, commit or PR records
that we asked. It is not a copy of derived state, and the board is still recomputed from
git on every single read. The distinction is enforced, not merely asserted: a run's column
is `state` (a run's lifecycle), never `status`, and `TestNoStatusColumn` fails the build
if any column name in the schema contains "status".

**The scheduler dispatches; it never merges.** `internal/schedule` has no path to
`ApproveMerge`, asserted by `TestSchedulerNeverMerges`, and `Fire` cannot reach GitHub's
merge endpoint at all (`TestFireAndApproveMergeAreSeparateCodePaths`). Unattended work
always stops at an open PR. Starting work an agent will open a PR for is recoverable;
landing code on main is not, so that step keeps its gate — a human, or the routine's own
auto-merge behind CI.

**Bounded by three numbers.** `FLIGHTDECK_MAX_PARALLEL` (default 2) caps tickets in flight
per project, counting both `in_progress` tickets and active runs the board cannot see yet.
`FLIGHTDECK_RUN_TIMEOUT` (default 30m) writes off a run whose branch never appeared.
`FLIGHTDECK_MAX_ATTEMPTS` (default 2) stops retrying and leaves the ticket for a human. A
failed dispatch burns an attempt too, so a misconfigured project stops rather than hammers.
Every one of these fails startup on a bad value: a typo must never fall back to a default
that spends money.

## Consequences
- The product does what it was for. Flip autopilot, and dependencies unblock work that
  starts by itself.
- **The blast radius is real and should be sized before enabling.** `MAX_PARALLEL` × the
  number of autopilot-on projects is how many agent sessions can be running at once, with
  nobody watching. Cost tracking (audit §7, stage 4.2) does not exist yet, so there is no
  budget ceiling — only the parallelism cap. Start at 1 on one project.
- Two dispatch paths now write runs, which is what keeps them from fighting. Any third
  path added later must do the same or it will double-fire.
- `MaxAttempts` is per ticket, forever — not per hour. A ticket that burns its attempts
  stays untouched until a human intervenes. That is deliberate: an agent that failed twice
  on the same ticket has a problem retrying will not fix.
- Unsolved: **two ready tickets that touch the same files.** The dependency graph orders
  what must be ordered, but knows nothing about file overlap, so parallel agents can
  produce a merge conflict on the second PR. Mitigated by the parallelism cap and by
  decomposition quality; not solved. It is the main reason `MAX_PARALLEL` defaults to 2
  rather than something ambitious.
