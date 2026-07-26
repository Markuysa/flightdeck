---
id: 20
title: Demo seed writes .claude/autopilot.json so the toggle works
role: backend
depends: [15, 17]
status: done
---
The demo project's seeded repo has no `.claude/autopilot.json`, so reading/flipping autopilot
(GET/PUT /api/projects/demo/autopilot) fails and the Fleet toggle errors. Seed the file so
autopilot is fully demoable end to end.

## Acceptance criteria
- [ ] `internal/demo` writes a valid `.claude/autopilot.json` (enabled:false, a maxInFlight, a
      note) into the seeded repo and commits it, so GET/PUT autopilot succeed for `demo`.
- [ ] The dispatcher's Autopilot/SetAutopilot round-trips against the seeded repo (read Off,
      flip On, read On) — proven by a test.
- [ ] lint + tests pass; no change to non-demo behaviour.

## Handoff

`internal/demo/buildRepo` now writes and commits `.claude/autopilot.json` (`enabled:false`,
`maxInFlight:1`, a note) into the seeded repo, so `GET/PUT /api/projects/demo/autopilot` and the
Fleet autopilot toggle work against the demo project end to end. `TestSeed_AutopilotRoundTrips`
proves the dispatcher reads Off → flips On → reads On against the seeded repo (local file ops,
no network). Non-demo behaviour is unchanged.
